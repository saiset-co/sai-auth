package main

import (
	"context"
	"github.com/saiset-co/sai-auth/pkg/providers"
	"log"

	"github.com/saiset-co/sai-auth/internal/handlers"
	"github.com/saiset-co/sai-auth/internal/repository"
	"github.com/saiset-co/sai-auth/internal/service"
	"github.com/saiset-co/sai-auth/internal/storage"
	"github.com/saiset-co/sai-auth/types"

	"github.com/saiset-co/sai-service/sai"
	saiService "github.com/saiset-co/sai-service/service"
)

func main() {
	ctx := context.Background()

	srv, err := saiService.NewService(ctx, "config.yml")
	if err != nil {
		log.Fatal("Failed to create service:", err)
	}

	config := sai.Config()

	var authConfig types.SaiAuthConfig
	config.GetAs("sai-auth", &authConfig)

	userRepo := storage.NewMongoUserRepository()
	roleRepo := repository.NewMongoRoleRepository()
	tokenRepo := storage.NewMongoTokenRepository()

	repos := &repository.Repositories{
		User:  userRepo,
		Role:  roleRepo,
		Token: tokenRepo,
	}

	authServiceURL := sai.Config().GetValue("auth_providers.sai-auth.params.auth_service_url", "http://localhost:8080").(string)

	authProvider := providers.NewSaiAuthProvider(config.GetConfig().Name, authServiceURL)
	if err := sai.RegisterAuthProvider("sai-auth", authProvider); err != nil {
		log.Fatal("Failed to register auth provider:", err)
	}
	permissionSvc := service.NewPermissionService(repos.Role)
	authSvc := service.NewAuthService(repos.User, repos.Role, repos.Token, permissionSvc, &authConfig)
	userSvc := service.NewUserService(repos.User, repos.Token, permissionSvc)
	userSvc.SetAuthService(authSvc)
	roleSvc := service.NewRoleService(repos.Role, repos.User, permissionSvc, userSvc)

	authHandler := handlers.NewAuthHandler(authSvc)
	userHandler := handlers.NewUserHandler(userSvc)
	roleHandler := handlers.NewRoleHandler(roleSvc)

	router := sai.Router()

	authGroup := router.Group("/api/v1/auth")
	authGroup.POST("/login", authHandler.Login).
		WithDoc("Login", "Authenticate user and get tokens", "Authentication", nil, nil).
		WithoutMiddlewares("auth")
	authGroup.POST("/refresh", authHandler.RefreshToken).
		WithDoc("Refresh Token", "Refresh access token", "Authentication", nil, nil).
		WithoutMiddlewares("auth")
	authGroup.POST("/logout", authHandler.Logout).
		WithDoc("Logout", "Logout and invalidate tokens", "Authentication", nil, nil).
		WithoutMiddlewares("auth")
	authGroup.GET("/me", authHandler.GetUserInfo).
		WithDoc("Get User Info", "Get current user information", "Authentication", nil, nil)
	authGroup.POST("/verify", authHandler.VerifyToken).
		WithDoc("Verify Token", "Verify token and permissions", "Authentication", nil, nil).
		WithoutMiddlewares("auth")

	userGroup := router.Group("/api/v1/users")
	userGroup.GET("/", userHandler.Get).
		WithDoc("Get Users", "Get users list", "Users", nil, nil)
	userGroup.POST("/", userHandler.Create).
		WithDoc("Create User", "Create new user", "Users", nil, nil)
	userGroup.PUT("/", userHandler.Update).
		WithDoc("Update User", "Update user", "Users", nil, nil)
	userGroup.DELETE("/", userHandler.Delete).
		WithDoc("Delete User", "Delete user", "Users", nil, nil)
	userGroup.POST("/assign-roles", userHandler.AssignRoles).
		WithDoc("Assign Roles", "Assign roles to user", "Users", nil, nil)
	userGroup.POST("/remove-roles", userHandler.RemoveRoles).
		WithDoc("Remove Roles", "Remove roles from user", "Users", nil, nil)

	roleGroup := router.Group("/api/v1/roles")
	roleGroup.GET("/", roleHandler.Get).
		WithDoc("Get Roles", "Get roles list", "Roles", nil, nil)
	roleGroup.POST("/", roleHandler.Create).
		WithDoc("Create Role", "Create new role", "Roles", nil, nil)
	roleGroup.PUT("/", roleHandler.Update).
		WithDoc("Update Role", "Update role", "Roles", nil, nil)
	roleGroup.DELETE("/", roleHandler.Delete).
		WithDoc("Delete Role", "Delete role", "Roles", nil, nil)
	roleGroup.GET("/permissions", roleHandler.GetPermissions).
		WithDoc("Get Role Permissions", "Get compiled role permissions", "Roles", nil, nil)
	roleGroup.POST("/permissions", authHandler.TestPermissions).
		WithDoc("Test Permissions", "Test user permissions", "Roles", nil, nil)

	if err := srv.Start(); err != nil {
		log.Fatal("Failed to start service:", err)
	}
}
