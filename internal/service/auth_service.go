package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/internal/repository"
	"github.com/saiset-co/sai-auth/types"
	saiTypes "github.com/saiset-co/sai-service/types"
)

type AuthService struct {
	userRepo      repository.UserRepository
	roleRepo      repository.RoleRepository
	tokenRepo     repository.TokenRepository
	permissionSvc *PermissionService
	config        *types.SaiAuthConfig
}

func NewAuthService(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	tokenRepo repository.TokenRepository,
	permissionSvc *PermissionService,
	config *types.SaiAuthConfig,
) *AuthService {
	return &AuthService{
		userRepo:      userRepo,
		roleRepo:      roleRepo,
		tokenRepo:     tokenRepo,
		permissionSvc: permissionSvc,
		config:        config,
	}
}

func (s *AuthService) Login(ctx *saiTypes.RequestCtx, req *models.LoginRequest) (*models.AuthResponse, error) {
	user, err := s.findUser(ctx, req.User)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if !user.IsActive {
		return nil, fmt.Errorf("user account is inactive")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if len(user.Roles) == 0 && !user.IsSuperUser {
		return nil, fmt.Errorf("user has no roles assigned")
	}

	existingToken, err := s.tokenRepo.GetByUserID(ctx, user.InternalID)
	if err == nil && existingToken != nil && existingToken.ExpiresAt > time.Now().UnixNano() {
		if req.Renew {
			permissions, err := s.permissionSvc.CompilePermissions(ctx, user)
			if err != nil {
				return nil, fmt.Errorf("failed to compile permissions: %w", err)
			}
			existingToken.CompiledPermissions = permissions
			err = s.tokenRepo.Update(ctx, existingToken)
			if err != nil {
				return nil, fmt.Errorf("failed to update token: %w", err)
			}
		}
		
		user.PasswordHash = ""
		return &models.AuthResponse{
			User: user,
			Tokens: &models.TokenResponse{
				AccessToken:  existingToken.AccessToken,
				RefreshToken: existingToken.RefreshToken,
				ExpiresIn:    (existingToken.ExpiresAt - time.Now().UnixNano()) / int64(time.Second),
			},
			Permissions: existingToken.CompiledPermissions,
		}, nil
	}

	s.tokenRepo.DeleteByUserID(ctx, user.InternalID)

	permissions, err := s.permissionSvc.CompilePermissions(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to compile permissions: %w", err)
	}

	token, err := s.generateToken(user.InternalID, permissions)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	err = s.tokenRepo.Store(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to store token: %w", err)
	}

	user.PasswordHash = ""

	return &models.AuthResponse{
		User: user,
		Tokens: &models.TokenResponse{
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			ExpiresIn:    (token.ExpiresAt - time.Now().UnixNano()) / int64(time.Second),
		},
		Permissions: permissions,
	}, nil
}

func (s *AuthService) findUser(ctx *saiTypes.RequestCtx, user string) (*models.User, error) {
	if strings.Contains(user, "@") {
		return s.userRepo.GetByEmail(ctx, user)
	}
	return s.userRepo.GetByUsername(ctx, user)
}

func (s *AuthService) RefreshToken(ctx *saiTypes.RequestCtx, req *models.RefreshTokenRequest) (*models.TokenResponse, error) {
	token, err := s.tokenRepo.GetByRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	user, err := s.userRepo.GetByID(ctx, token.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if !user.IsActive {
		s.tokenRepo.Delete(ctx, token.InternalID)
		return nil, fmt.Errorf("user account is inactive")
	}

	permissions, err := s.permissionSvc.CompilePermissions(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to compile permissions: %w", err)
	}

	newToken, err := s.generateToken(user.InternalID, permissions)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	s.tokenRepo.Delete(ctx, token.InternalID)

	err = s.tokenRepo.Store(ctx, newToken)
	if err != nil {
		return nil, fmt.Errorf("failed to store token: %w", err)
	}

	return &models.TokenResponse{
		AccessToken:  newToken.AccessToken,
		RefreshToken: newToken.RefreshToken,
		ExpiresIn:    (newToken.ExpiresAt - time.Now().UnixNano()) / int64(time.Second),
	}, nil
}

func (s *AuthService) Logout(ctx *saiTypes.RequestCtx, accessToken string) error {
	token, err := s.tokenRepo.GetByAccessToken(ctx, accessToken)
	if err != nil {
		return nil
	}

	return s.tokenRepo.Delete(ctx, token.InternalID)
}

func (s *AuthService) GetUserInfo(ctx *saiTypes.RequestCtx, accessToken string) (*models.UserInfoResponse, error) {
	token, err := s.tokenRepo.GetByAccessToken(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	user, err := s.userRepo.GetByID(ctx, token.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	user.PasswordHash = ""

	return &models.UserInfoResponse{
		User:        user,
		Permissions: token.CompiledPermissions,
	}, nil
}

func (s *AuthService) VerifyToken(ctx *saiTypes.RequestCtx, req *models.VerifyRequest) (*models.VerifyResponse, error) {
	userCount, err := s.userRepo.CountUsers(ctx)
	if err == nil && userCount == 0 {
		return &models.VerifyResponse{
			Allowed:        true,
			UserID:         "no-users",
			ModifiedParams: req.RequestParams,
			Reason:         "No users in system - access granted",
		}, nil
	}

	token, err := s.tokenRepo.GetByAccessToken(ctx, req.Token)
	if err != nil {
		return &models.VerifyResponse{
			Allowed: false,
			Reason:  "Invalid or expired token",
		}, nil
	}

	user, err := s.userRepo.GetByID(ctx, token.UserID)
	if err != nil {
		return &models.VerifyResponse{
			Allowed: false,
			Reason:  "User not found",
		}, nil
	}

	if !user.IsActive {
		return &models.VerifyResponse{
			Allowed: false,
			Reason:  "User account is inactive",
		}, nil
	}

	if user.IsSuperUser {
		modifiedParams := make(map[string]interface{})
		for key, value := range req.RequestParams {
			modifiedParams[key] = value
		}

		return &models.VerifyResponse{
			Allowed:        true,
			UserID:         user.InternalID,
			ModifiedParams: modifiedParams,
		}, nil
	}

	result, err := s.permissionSvc.CheckPermission(
		ctx,
		token.CompiledPermissions,
		req.Microservice,
		req.Method,
		req.Path,
		req.RequestParams,
	)
	if err != nil {
		return &models.VerifyResponse{
			Allowed: false,
			Reason:  fmt.Sprintf("Permission check failed: %v", err),
		}, nil
	}

	result.UserID = user.InternalID
	return result, nil
}

func (s *AuthService) TestPermissions(ctx *saiTypes.RequestCtx, req *models.TestPermissionsRequest) (*models.VerifyResponse, error) {
	user, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if user.IsSuperUser {
		modifiedParams := make(map[string]interface{})
		for key, value := range req.TestParams {
			modifiedParams[key] = value
		}

		return &models.VerifyResponse{
			Allowed:        true,
			UserID:         user.InternalID,
			ModifiedParams: modifiedParams,
		}, nil
	}

	permissions, err := s.permissionSvc.CompilePermissions(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to compile permissions: %w", err)
	}

	result, err := s.permissionSvc.CheckPermission(
		ctx,
		permissions,
		req.Microservice,
		req.Method,
		req.Path,
		req.TestParams,
	)
	if err != nil {
		return &models.VerifyResponse{
			Allowed: false,
			Reason:  fmt.Sprintf("Permission check failed: %v", err),
		}, nil
	}

	result.UserID = user.InternalID
	return result, nil
}

func (s *AuthService) generateToken(userID string, permissions []models.CompiledPermission) (*models.Token, error) {
	accessToken, err := s.generateRandomString(64)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateRandomString(64)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	return &models.Token{
		UserID:              userID,
		AccessToken:         accessToken,
		RefreshToken:        refreshToken,
		ExpiresAt:           now.Add(s.config.AccessTokenTTL).UnixNano(),
		RefreshExpiresAt:    now.Add(s.config.RefreshTokenTTL).UnixNano(),
		CompiledPermissions: permissions,
	}, nil
}

func (s *AuthService) generateRandomString(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *AuthService) isSuperUser(ctx *saiTypes.RequestCtx, user *models.User) bool {
	return user.IsSuperUser
}

func (s *AuthService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.config.BcryptCost)
	return string(hash), err
}
