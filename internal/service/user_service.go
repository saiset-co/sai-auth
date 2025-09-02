package service

import (
	"fmt"
	"github.com/google/uuid"

	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/internal/repository"
	"github.com/saiset-co/sai-auth/types"
	saiTypes "github.com/saiset-co/sai-service/types"
)

type UserService struct {
	userRepo      repository.UserRepository
	tokenRepo     repository.TokenRepository
	permissionSvc *PermissionService
	authService   *AuthService
}

func NewUserService(
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	permissionSvc *PermissionService,
) *UserService {
	return &UserService{
		userRepo:      userRepo,
		tokenRepo:     tokenRepo,
		permissionSvc: permissionSvc,
	}
}

func (s *UserService) SetAuthService(authService *AuthService) {
	s.authService = authService
}

func (s *UserService) Create(ctx *saiTypes.RequestCtx, req *models.CreateUserRequest) (*models.User, error) {
	_, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err == nil {
		return nil, fmt.Errorf("username already exists")
	}

	_, err = s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil {
		return nil, fmt.Errorf("email already exists")
	}

	passwordHash, err := s.authService.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	userCount, err := s.userRepo.CountUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	user := &models.User{
		InternalID:   uuid.New().String(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		IsActive:     true,
		IsSuperUser:  userCount == 0,
		Roles:        []string{},
		Data:         req.Data,
	}

	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	if user.Data == nil {
		user.Data = make(map[string]interface{})
	}

	err = s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	user.PasswordHash = ""
	return user, nil
}

func (s *UserService) GetByID(ctx *saiTypes.RequestCtx, id string) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	user.PasswordHash = ""
	return user, nil
}

func (s *UserService) List(ctx *saiTypes.RequestCtx, filter *types.UserFilterRequest) ([]*models.User, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	users, total, err := s.userRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	for _, user := range users {
		user.PasswordHash = ""
		user.IsSuperUser = false
	}

	return users, total, nil
}

func (s *UserService) Update(ctx *saiTypes.RequestCtx, filter, data map[string]interface{}) error {
	hasOperators := false
	rolesUpdated := false

	for key := range data {
		if len(key) > 0 && key[0] == '$' {
			hasOperators = true
			break
		}
	}

	var updateData map[string]interface{}

	if hasOperators {
		updateData = data

		for _, opValue := range data {
			if opMap, ok := opValue.(map[string]interface{}); ok {
				if _, exists := opMap["is_super_user"]; exists {
					delete(opMap, "is_super_user")
				}
				if _, exists := opMap["IsSuperUser"]; exists {
					delete(opMap, "IsSuperUser")
				}

				if _, exists := opMap["roles"]; exists {
					rolesUpdated = true
				}

				if password, exists := opMap["password"]; exists {
					if passwordStr, ok := password.(string); ok {
						hashedPassword, err := s.authService.HashPassword(passwordStr)
						if err != nil {
							return fmt.Errorf("failed to hash password: %w", err)
						}
						opMap["password_hash"] = hashedPassword
						delete(opMap, "password")
					}
				}
			}
		}
	} else {
		if password, exists := data["password"]; exists {
			if passwordStr, ok := password.(string); ok {
				hashedPassword, err := s.authService.HashPassword(passwordStr)
				if err != nil {
					return fmt.Errorf("failed to hash password: %w", err)
				}
				data["password_hash"] = hashedPassword
				delete(data, "password")
			}
		}

		updateData = map[string]interface{}{"$set": data}

		if _, exists := data["roles"]; exists {
			rolesUpdated = true
		}

		if _, exists := data["is_super_user"]; exists {
			delete(data, "is_super_user")
		}
		if _, exists := data["IsSuperUser"]; exists {
			delete(data, "IsSuperUser")
		}
	}

	err := s.userRepo.Update(ctx, filter, updateData)
	if err != nil {
		return err
	}

	if rolesUpdated {
		return s.recompileUserPermissions(ctx, filter)
	}

	return nil
}

func (s *UserService) Delete(ctx *saiTypes.RequestCtx, filter map[string]interface{}) error {
	users, _, err := s.userRepo.List(ctx, &types.UserFilterRequest{})
	if err != nil {
		return err
	}

	for _, user := range users {
		if s.matchesFilter(user, filter) {
			s.tokenRepo.DeleteByUserID(ctx, user.InternalID)
		}
	}

	return s.userRepo.Delete(ctx, filter)
}

func (s *UserService) AssignRoles(ctx *saiTypes.RequestCtx, userID string, roleIDs []string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	roleMap := make(map[string]bool)
	for _, roleID := range user.Roles {
		roleMap[roleID] = true
	}

	for _, roleID := range roleIDs {
		roleMap[roleID] = true
	}

	newRoles := make([]string, 0, len(roleMap))
	for roleID := range roleMap {
		newRoles = append(newRoles, roleID)
	}

	if len(newRoles) > 10 {
		return fmt.Errorf("maximum 10 roles per user exceeded")
	}

	err = s.userRepo.Update(ctx,
		map[string]interface{}{"internal_id": userID},
		map[string]interface{}{"$set": map[string]interface{}{"roles": newRoles}},
	)
	if err != nil {
		return err
	}

	return s.recompileUserPermissions(ctx, map[string]interface{}{"internal_id": userID})
}

func (s *UserService) RemoveRoles(ctx *saiTypes.RequestCtx, userID string, roleIDs []string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	removeMap := make(map[string]bool)
	for _, roleID := range roleIDs {
		removeMap[roleID] = true
	}

	newRoles := make([]string, 0)
	for _, roleID := range user.Roles {
		if !removeMap[roleID] {
			newRoles = append(newRoles, roleID)
		}
	}

	err = s.userRepo.Update(ctx,
		map[string]interface{}{"internal_id": userID},
		map[string]interface{}{"$set": map[string]interface{}{"roles": newRoles}},
	)
	if err != nil {
		return err
	}

	return s.recompileUserPermissions(ctx, map[string]interface{}{"internal_id": userID})
}

func (s *UserService) recompileUserPermissions(ctx *saiTypes.RequestCtx, filter map[string]interface{}) error {
	users, _, err := s.userRepo.List(ctx, &types.UserFilterRequest{})
	if err != nil {
		return err
	}

	for _, user := range users {
		if s.matchesFilter(user, filter) {
			token, err := s.tokenRepo.GetByUserID(ctx, user.InternalID)
			if err != nil {
				continue
			}

			permissions, err := s.permissionSvc.CompilePermissions(ctx, user)
			if err != nil {
				continue
			}

			token.CompiledPermissions = permissions

			s.tokenRepo.Update(ctx, token)
		}
	}

	return nil
}

func (s *UserService) matchesFilter(user *models.User, filter map[string]interface{}) bool {
	for key, value := range filter {
		switch key {
		case "internal_id":
			if user.InternalID != value {
				return false
			}
		case "username":
			if user.Username != value {
				return false
			}
		case "email":
			if user.Email != value {
				return false
			}
		case "is_active":
			if user.IsActive != value {
				return false
			}
		}
	}
	return true
}
