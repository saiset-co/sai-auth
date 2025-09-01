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

type RoleHandler struct {
	roleService *service.RoleService
}

func NewRoleHandler(roleService *service.RoleService) *RoleHandler {
	return &RoleHandler{
		roleService: roleService,
	}
}

func (h *RoleHandler) Create(ctx *saiTypes.RequestCtx) {
	var req models.CreateRoleRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.Error(err, fasthttp.StatusBadRequest)
		return
	}

	if req.Name == "" {
		ctx.Error(errors.New("Role name is required"), fasthttp.StatusBadRequest)
		return
	}

	role, err := h.roleService.Create(ctx, &req)
	if err != nil {
		if err.Error() == "role name already exists" {
			ctx.Error(err, fasthttp.StatusConflict)
		} else {
			ctx.Error(err, fasthttp.StatusInternalServerError)
		}
		return
	}

	response := types.Response{
		Data:    role,
		Created: 1,
	}

	ctx.SuccessJSON(response)
}

func (h *RoleHandler) Get(ctx *saiTypes.RequestCtx) {
	filter := h.parseFilter(ctx)

	page, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("page")))
	limit, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("limit")))
	search := string(ctx.QueryArgs().Peek("search"))

	var active *bool
	if activeStr := string(ctx.QueryArgs().Peek("active")); activeStr != "" {
		if activeBool, err := strconv.ParseBool(activeStr); err == nil {
			active = &activeBool
		}
	}

	filterReq := &types.RoleFilterRequest{
		PaginationRequest: types.PaginationRequest{
			Page:   page,
			Limit:  limit,
			Search: search,
		},
		Active: active,
	}

	if len(filter) > 0 {
		if roleID, exists := filter["internal_id"]; exists {
			role, err := h.roleService.GetByID(ctx, roleID.(string))
			if err != nil {
				ctx.Error(err, fasthttp.StatusNotFound)
				return
			}

			ctx.SuccessJSON([]*models.Role{role})
			return
		}

		if roleName, exists := filter["name"]; exists {
			role, err := h.roleService.GetByName(ctx, roleName.(string))
			if err != nil {
				ctx.Error(err, fasthttp.StatusNotFound)
				return
			}

			ctx.SuccessJSON([]*models.Role{role})
			return
		}
	}

	roles, total, err := h.roleService.List(ctx, filterReq)
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
		Data:       roles,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}

	ctx.SuccessJSON(response)
}

func (h *RoleHandler) Update(ctx *saiTypes.RequestCtx) {
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

	err := h.roleService.Update(ctx, req.Filter, req.Data)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	response := types.Response{
		Updated: 1,
	}

	ctx.SuccessJSON(response)
}

func (h *RoleHandler) Delete(ctx *saiTypes.RequestCtx) {
	var req types.DeleteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.Error(err, fasthttp.StatusBadRequest)
		return
	}

	if len(req.Filter) == 0 {
		ctx.Error(errors.New("Filter is required"), fasthttp.StatusBadRequest)
		return
	}

	err := h.roleService.Delete(ctx, req.Filter)
	if err != nil {
		ctx.Error(err, fasthttp.StatusInternalServerError)
		return
	}

	response := types.Response{
		Deleted: 1,
	}

	ctx.SuccessJSON(response)
}

func (h *RoleHandler) GetPermissions(ctx *saiTypes.RequestCtx) {
	roleID := string(ctx.QueryArgs().Peek("role_id"))
	if roleID == "" {
		ctx.Error(errors.New("role_id is required"), fasthttp.StatusBadRequest)
		return
	}

	permissions, err := h.roleService.GetRolePermissions(ctx, roleID)
	if err != nil {
		ctx.Error(err, fasthttp.StatusNotFound)
		return
	}

	ctx.SuccessJSON(permissions)
}

func (h *RoleHandler) parseFilter(ctx *saiTypes.RequestCtx) map[string]interface{} {
	filter := make(map[string]interface{})

	if body := ctx.PostBody(); len(body) > 0 {
		var getReq types.GetRequest
		if err := ctx.ReadJSON(&getReq); err == nil && len(getReq.Filter) > 0 {
			return getReq.Filter
		}
	}

	return filter
}
