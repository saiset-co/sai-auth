package handlers

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/internal/service"
	"github.com/saiset-co/sai-service/sai"
	saiTypes "github.com/saiset-co/sai-service/types"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) Login(ctx *saiTypes.RequestCtx) {
	var req models.LoginRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.Error(err, fasthttp.StatusBadRequest)
		return
	}

	if req.User == "" || req.Password == "" {
		ctx.Error(errors.New("User and password are required"), fasthttp.StatusBadRequest)
		return
	}

	response, err := h.authService.Login(ctx, &req)
	if err != nil {
		ctx.Error(err, fasthttp.StatusUnauthorized)
		return
	}

	ctx.SuccessJSON(response)
}

func (h *AuthHandler) RefreshToken(ctx *saiTypes.RequestCtx) {
	var req models.RefreshTokenRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.Error(err, fasthttp.StatusBadRequest)
		return
	}

	if req.RefreshToken == "" {
		ctx.Error(errors.New("Refresh token is required"), fasthttp.StatusBadRequest)
		return
	}

	response, err := h.authService.RefreshToken(ctx, &req)
	if err != nil {
		ctx.Error(err, fasthttp.StatusUnauthorized)
		return
	}

	ctx.SuccessJSON(response)
}

func (h *AuthHandler) Logout(ctx *saiTypes.RequestCtx) {
	token := h.extractToken(ctx)
	if token == "" {
		ctx.Error(errors.New("Authorization token required"), fasthttp.StatusUnauthorized)
		return
	}

	err := h.authService.Logout(ctx, token)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	ctx.SuccessJSON(map[string]string{"message": "Logged out successfully"})
}

func (h *AuthHandler) GetUserInfo(ctx *saiTypes.RequestCtx) {
	token := h.extractToken(ctx)
	if token == "" {
		ctx.Error(errors.New("Authorization token required"), fasthttp.StatusUnauthorized)
		return
	}

	response, err := h.authService.GetUserInfo(ctx, token)
	if err != nil {
		ctx.Error(err, fasthttp.StatusUnauthorized)
		return
	}

	ctx.SuccessJSON(response)
}

func (h *AuthHandler) VerifyToken(ctx *saiTypes.RequestCtx) {
	var req models.VerifyRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.Error(err, fasthttp.StatusBadRequest)
		return
	}

	if req.Microservice == "" || req.Method == "" || req.Path == "" {
		ctx.Error(errors.New("Microservice, method, and path are required"), fasthttp.StatusBadRequest)
		return
	}

	response, err := h.authService.VerifyToken(ctx, &req)
	if err != nil {
		sai.Logger().Error("Auth verify error", zap.Error(err))
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	if !response.Allowed {
		sai.Logger().Warn("Auth verify denied",
			zap.String("microservice", req.Microservice),
			zap.String("method", req.Method),
			zap.String("path", req.Path),
			zap.Any("request_params", req.RequestParams),
			zap.String("reason", response.Reason))
		ctx.Error(errors.New("Not allowed"), fasthttp.StatusForbidden)
		return
	}

	ctx.SuccessJSON(response)
}

func (h *AuthHandler) TestPermissions(ctx *saiTypes.RequestCtx) {
	var req models.TestPermissionsRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.Error(err, fasthttp.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.Microservice == "" || req.Method == "" || req.Path == "" {
		ctx.Error(errors.New("UserID, microservice, method, and path are required"), fasthttp.StatusBadRequest)
		return
	}

	response, err := h.authService.TestPermissions(ctx, &req)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	ctx.SuccessJSON(response)
}

func (h *AuthHandler) extractToken(ctx *saiTypes.RequestCtx) string {
	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	if strings.HasPrefix(authHeader, "Token ") {
		return strings.TrimPrefix(authHeader, "Token ")
	}
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}
