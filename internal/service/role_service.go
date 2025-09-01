package service

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/internal/repository"
	"github.com/saiset-co/sai-auth/types"
	saiTypes "github.com/saiset-co/sai-service/types"
)

type RoleService struct {
	roleRepo      repository.RoleRepository
	userRepo      repository.UserRepository
	permissionSvc *PermissionService
	userService   *UserService
}

func NewRoleService(
	roleRepo repository.RoleRepository,
	userRepo repository.UserRepository,
	permissionSvc *PermissionService,
	userService *UserService,
) *RoleService {
	return &RoleService{
		roleRepo:      roleRepo,
		userRepo:      userRepo,
		permissionSvc: permissionSvc,
		userService:   userService,
	}
}

func (s *RoleService) Create(ctx *saiTypes.RequestCtx, req *models.CreateRoleRequest) (*models.Role, error) {
	_, err := s.roleRepo.GetByName(ctx, req.Name)
	if err == nil {
		return nil, fmt.Errorf("role name already exists")
	}

	if len(req.Permissions) > 50 {
		return nil, fmt.Errorf("maximum 50 permissions per role exceeded")
	}

	role := &models.Role{
		InternalID:  uuid.New().String(),
		Name:        req.Name,
		IsActive:    true,
		ParentRoles: req.ParentRoles,
		Permissions: req.Permissions,
		Data:        req.Data,
	}

	if req.IsActive != nil {
		role.IsActive = *req.IsActive
	}

	if role.Data == nil {
		role.Data = make(map[string]interface{})
	}

	if err := s.validateRoleHierarchy(ctx, role.InternalID, req.ParentRoles, 0); err != nil {
		return nil, err
	}

	err = s.roleRepo.Create(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	return role, nil
}

func (s *RoleService) GetByID(ctx *saiTypes.RequestCtx, id string) (*models.Role, error) {
	return s.roleRepo.GetByID(ctx, id)
}

func (s *RoleService) GetByName(ctx *saiTypes.RequestCtx, name string) (*models.Role, error) {
	return s.roleRepo.GetByName(ctx, name)
}

func (s *RoleService) List(ctx *saiTypes.RequestCtx, filter *types.RoleFilterRequest) ([]*models.Role, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	return s.roleRepo.List(ctx, filter)
}

func (s *RoleService) Update(ctx *saiTypes.RequestCtx, filter, data map[string]interface{}) error {
	roles, _, err := s.roleRepo.List(ctx, &types.RoleFilterRequest{})
	if err != nil {
		return err
	}

	var affectedRoles []*models.Role
	for _, role := range roles {
		if s.matchesFilter(role, filter) {
			affectedRoles = append(affectedRoles, role)
		}
	}

	if permissions, exists := data["permissions"]; exists {
		if permSlice, ok := permissions.([]models.Permission); ok && len(permSlice) > 50 {
			return fmt.Errorf("maximum 50 permissions per role exceeded")
		}
	}

	if parentRoles, exists := data["parent_roles"]; exists {
		if parentSlice, ok := parentRoles.([]string); ok {
			for _, role := range affectedRoles {
				if err := s.validateRoleHierarchy(ctx, role.InternalID, parentSlice, 0); err != nil {
					return err
				}
			}
		}
	}

	err = s.roleRepo.Update(ctx, filter, map[string]interface{}{"$set": data})
	if err != nil {
		return err
	}

	for _, role := range affectedRoles {
		s.recompileRolePermissions(ctx, role.InternalID)
	}

	return nil
}

func (s *RoleService) Delete(ctx *saiTypes.RequestCtx, filter map[string]interface{}) error {
	roles, _, err := s.roleRepo.List(ctx, &types.RoleFilterRequest{})
	if err != nil {
		return err
	}

	var rolesToDelete []string
	for _, role := range roles {
		if s.matchesFilter(role, filter) {
			rolesToDelete = append(rolesToDelete, role.InternalID)
		}
	}

	for _, roleID := range rolesToDelete {
		users, _, err := s.userRepo.List(ctx, &types.UserFilterRequest{})
		if err != nil {
			continue
		}

		for _, user := range users {
			hasRole := false
			for _, userRoleID := range user.Roles {
				if userRoleID == roleID {
					hasRole = true
					break
				}
			}

			if hasRole {
				newRoles := make([]string, 0)
				for _, userRoleID := range user.Roles {
					if userRoleID != roleID {
						newRoles = append(newRoles, userRoleID)
					}
				}

				s.userRepo.Update(ctx,
					map[string]interface{}{"internal_id": user.InternalID},
					map[string]interface{}{"$set": map[string]interface{}{"roles": newRoles}},
				)

				s.userService.recompileUserPermissions(ctx, map[string]interface{}{"internal_id": user.InternalID})
			}
		}
	}

	return s.roleRepo.Delete(ctx, filter)
}

func (s *RoleService) GetRolePermissions(ctx *saiTypes.RequestCtx, roleID string) (*models.RolePermissionsResponse, error) {
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return nil, err
	}

	userIDs, err := s.roleRepo.GetUsersByRole(ctx, roleID)
	if err != nil {
		return nil, err
	}

	dummyUser := &models.User{
		InternalID: "dummy",
		Roles:      []string{roleID},
		Data:       make(map[string]interface{}),
	}

	permissions, err := s.permissionSvc.CompilePermissions(ctx, dummyUser)
	if err != nil {
		return nil, err
	}

	return &models.RolePermissionsResponse{
		Role: models.RoleInfo{
			InternalID: role.InternalID,
			Name:       role.Name,
		},
		Users:       userIDs,
		Permissions: permissions,
	}, nil
}

func (s *RoleService) validateRoleHierarchy(ctx *saiTypes.RequestCtx, roleID string, parentRoles []string, depth int) error {
	if depth > 5 {
		return fmt.Errorf("maximum role inheritance depth (5) exceeded")
	}

	for _, parentRoleID := range parentRoles {
		if parentRoleID == roleID {
			return fmt.Errorf("circular role dependency detected")
		}

		parentRole, err := s.roleRepo.GetByID(ctx, parentRoleID)
		if err != nil {
			return fmt.Errorf("parent role %s not found", parentRoleID)
		}

		if !parentRole.IsActive {
			return fmt.Errorf("parent role %s is inactive", parentRoleID)
		}

		if err := s.validateRoleHierarchy(ctx, roleID, parentRole.ParentRoles, depth+1); err != nil {
			return err
		}
	}

	return nil
}

func (s *RoleService) recompileRolePermissions(ctx *saiTypes.RequestCtx, roleID string) {
	users, _, err := s.userRepo.List(ctx, &types.UserFilterRequest{})
	if err != nil {
		return
	}

	for _, user := range users {
		hasRole := false
		for _, userRoleID := range user.Roles {
			if userRoleID == roleID {
				hasRole = true
				break
			}
		}

		if hasRole {
			s.userService.recompileUserPermissions(ctx, map[string]interface{}{"internal_id": user.InternalID})
		}
	}
}

func (s *RoleService) matchesFilter(role *models.Role, filter map[string]interface{}) bool {
	for key, value := range filter {
		switch key {
		case "internal_id":
			if role.InternalID != value {
				return false
			}
		case "name":
			if role.Name != value {
				return false
			}
		case "is_active":
			if role.IsActive != value {
				return false
			}
		}
	}
	return true
}
