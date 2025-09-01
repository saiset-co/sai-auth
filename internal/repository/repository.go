package repository

import (
	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/types"
	saiTypes "github.com/saiset-co/sai-service/types"
)

type UserRepository interface {
	Create(ctx *saiTypes.RequestCtx, user *models.User) error
	GetByID(ctx *saiTypes.RequestCtx, id string) (*models.User, error)
	GetByUsername(ctx *saiTypes.RequestCtx, username string) (*models.User, error)
	GetByEmail(ctx *saiTypes.RequestCtx, email string) (*models.User, error)
	Update(ctx *saiTypes.RequestCtx, filter, data map[string]interface{}) error
	Delete(ctx *saiTypes.RequestCtx, filter map[string]interface{}) error
	List(ctx *saiTypes.RequestCtx, filter *types.UserFilterRequest) ([]*models.User, int64, error)
	GetFirstUser(ctx *saiTypes.RequestCtx) (*models.User, error)
	CountUsers(ctx *saiTypes.RequestCtx) (int64, error)
}

type RoleRepository interface {
	Create(ctx *saiTypes.RequestCtx, role *models.Role) error
	GetByID(ctx *saiTypes.RequestCtx, id string) (*models.Role, error)
	GetByName(ctx *saiTypes.RequestCtx, name string) (*models.Role, error)
	GetByIDs(ctx *saiTypes.RequestCtx, ids []string) ([]*models.Role, error)
	Update(ctx *saiTypes.RequestCtx, filter, data map[string]interface{}) error
	Delete(ctx *saiTypes.RequestCtx, filter map[string]interface{}) error
	List(ctx *saiTypes.RequestCtx, filter *types.RoleFilterRequest) ([]*models.Role, int64, error)
	GetUsersByRole(ctx *saiTypes.RequestCtx, roleID string) ([]string, error)
}

type TokenRepository interface {
	Store(ctx *saiTypes.RequestCtx, token *models.Token) error
	GetByAccessToken(ctx *saiTypes.RequestCtx, accessToken string) (*models.Token, error)
	GetByRefreshToken(ctx *saiTypes.RequestCtx, refreshToken string) (*models.Token, error)
	GetByUserID(ctx *saiTypes.RequestCtx, userID string) (*models.Token, error)
	Update(ctx *saiTypes.RequestCtx, token *models.Token) error
	Delete(ctx *saiTypes.RequestCtx, tokenID string) error
	DeleteByUserID(ctx *saiTypes.RequestCtx, userID string) error
	List(ctx *saiTypes.RequestCtx, filter *types.TokenFilterRequest) ([]*models.Token, int64, error)
	IsValid(ctx *saiTypes.RequestCtx, accessToken string) bool
}

type Repositories struct {
	User  UserRepository
	Role  RoleRepository
	Token TokenRepository
}
