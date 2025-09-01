package handlers

import (
	"math"
	"strconv"

	"github.com/pkg/errors"
	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/internal/service"
	"github.com/saiset-co/sai-auth/types"
	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/valyala/fasthttp"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

func (h *UserHandler) Create(ctx *saiTypes.RequestCtx) {
	var req models.CreateUserRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.Error(err, fasthttp.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		ctx.Error(errors.New("Username, email, and password are required"), fasthttp.StatusBadRequest)
		return
	}

	user, err := h.userService.Create(ctx, &req)
	if err != nil {
		if err.Error() == "username already exists" || err.Error() == "email already exists" {
			ctx.Error(err, fasthttp.StatusConflict)
		} else {
			ctx.Error(err, fasthttp.StatusInternalServerError)
		}
		return
	}

	response := types.Response{
		Data:    user,
		Created: 1,
	}

	ctx.SuccessJSON(response)
}

func (h *UserHandler) Get(ctx *saiTypes.RequestCtx) {
	filter := h.parseFilter(ctx)

	page, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("page")))
	limit, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("limit")))
	search := string(ctx.QueryArgs().Peek("search"))
	role := string(ctx.QueryArgs().Peek("role"))

	var active *bool
	if activeStr := string(ctx.QueryArgs().Peek("active")); activeStr != "" {
		if activeBool, err := strconv.ParseBool(activeStr); err == nil {
			active = &activeBool
		}
	}

	filterReq := &types.UserFilterRequest{
		PaginationRequest: types.PaginationRequest{
			Page:   page,
			Limit:  limit,
			Search: search,
		},
		Role:   role,
		Active: active,
	}

	if len(filter) > 0 {
		if userID, exists := filter["internal_id"]; exists {
			user, err := h.userService.GetByID(ctx, userID.(string))
			if err != nil {
				ctx.Error(err, fasthttp.StatusNotFound)
				return
			}

			ctx.SuccessJSON([]*models.User{user})
			return
		}
	}

	users, total, err := h.userService.List(ctx, filterReq)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	response := types.PaginatedResponse{
		Data:       users,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}

	ctx.SuccessJSON(response)
}

func (h *UserHandler) Update(ctx *saiTypes.RequestCtx) {
	var req types.UpdateRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.Error(err, fasthttp.StatusBadRequest)
		return
	}

	if len(req.Filter) == 0 {
		ctx.Error(errors.New("Filter is required"), fasthttp.StatusBadRequest)
		return
	}

	if len(req.Data) == 0 {
		ctx.Error(errors.New("Data is required"), fasthttp.StatusBadRequest)
		return
	}

	err := h.userService.Update(ctx, req.Filter, req.Data)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	response := types.Response{
		Updated: 1,
	}

	ctx.SuccessJSON(response)
}

func (h *UserHandler) Delete(ctx *saiTypes.RequestCtx) {
	var req types.DeleteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.Error(err, fasthttp.StatusBadRequest)
		return
	}

	if len(req.Filter) == 0 {
		ctx.Error(errors.New("Filter is required"), fasthttp.StatusBadRequest)
		return
	}

	err := h.userService.Delete(ctx, req.Filter)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	response := types.Response{
		Deleted: 1,
	}

	ctx.SuccessJSON(response)
}

func (h *UserHandler) AssignRoles(ctx *saiTypes.RequestCtx) {
	userID := string(ctx.QueryArgs().Peek("user_id"))
	if userID == "" {
		ctx.Error(errors.New("user_id is required"), fasthttp.StatusBadRequest)
		return
	}

	var req struct {
		RoleIDs []string `json:"role_ids"`
	}

	if err := ctx.ReadJSON(&req); err != nil {
		ctx.Error(err, fasthttp.StatusBadRequest)
		return
	}

	if len(req.RoleIDs) == 0 {
		ctx.Error(errors.New("role_ids are required"), fasthttp.StatusBadRequest)
		return
	}

	err := h.userService.AssignRoles(ctx, userID, req.RoleIDs)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	response := types.Response{
		Updated: 1,
	}

	ctx.SuccessJSON(response)
}

func (h *UserHandler) RemoveRoles(ctx *saiTypes.RequestCtx) {
	userID := string(ctx.QueryArgs().Peek("user_id"))
	if userID == "" {
		ctx.Error(errors.New("user_id is required"), fasthttp.StatusBadRequest)
		return
	}

	var req struct {
		RoleIDs []string `json:"role_ids"`
	}

	if err := ctx.ReadJSON(&req); err != nil {
		ctx.Error(err, fasthttp.StatusBadRequest)
		return
	}

	if len(req.RoleIDs) == 0 {
		ctx.Error(errors.New("role_ids are required"), fasthttp.StatusBadRequest)
		return
	}

	err := h.userService.RemoveRoles(ctx, userID, req.RoleIDs)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	response := types.Response{
		Updated: 1,
	}

	ctx.SuccessJSON(response)
}

func (h *UserHandler) parseFilter(ctx *saiTypes.RequestCtx) map[string]interface{} {
	filter := make(map[string]interface{})

	if body := ctx.PostBody(); len(body) > 0 {
		var getReq types.GetRequest
		if err := ctx.ReadJSON(&getReq); err == nil && len(getReq.Filter) > 0 {
			return getReq.Filter
		}
	}

	return filter
}
