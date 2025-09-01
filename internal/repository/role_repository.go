package repository

import (
	"fmt"

	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/types"
	"github.com/saiset-co/sai-service/sai"
	saiTypes "github.com/saiset-co/sai-service/types"
)

type MongoRoleRepository struct {
	client saiTypes.ClientManager
}

func NewMongoRoleRepository() RoleRepository {
	return &MongoRoleRepository{
		client: sai.ClientManager(),
	}
}

func (r *MongoRoleRepository) Create(ctx *saiTypes.RequestCtx, role *models.Role) error {
	reqData := map[string]interface{}{
		"collection": "roles",
		"data":       []interface{}{role},
	}

	_, _, err := r.client.Call("storage", "POST", "/api/v1/documents", reqData, nil)
	return err
}

func (r *MongoRoleRepository) GetByID(ctx *saiTypes.RequestCtx, id string) (*models.Role, error) {
	reqData := map[string]interface{}{
		"collection": "roles",
		"filter":     map[string]interface{}{"internal_id": id},
		"limit":      1,
	}

	response, statusCode, err := r.client.Call("storage", "GET", "/api/v1/documents", reqData, nil)
	if err != nil {
		return nil, err
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("storage request failed with status %d", statusCode)
	}

	var result struct {
		Data []models.Role `json:"data"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("role not found")
	}

	return &result.Data[0], nil
}

func (r *MongoRoleRepository) GetByName(ctx *saiTypes.RequestCtx, name string) (*models.Role, error) {
	reqData := map[string]interface{}{
		"collection": "roles",
		"filter":     map[string]interface{}{"name": name},
		"limit":      1,
	}

	response, statusCode, err := r.client.Call("storage", "GET", "/api/v1/documents", reqData, nil)
	if err != nil {
		return nil, err
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("storage request failed with status %d", statusCode)
	}

	var result struct {
		Data []models.Role `json:"data"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("role not found")
	}

	return &result.Data[0], nil
}

func (r *MongoRoleRepository) GetByIDs(ctx *saiTypes.RequestCtx, ids []string) ([]*models.Role, error) {
	if len(ids) == 0 {
		return []*models.Role{}, nil
	}

	reqData := map[string]interface{}{
		"collection": "roles",
		"filter":     map[string]interface{}{"internal_id": map[string]interface{}{"$in": ids}},
	}

	response, statusCode, err := r.client.Call("storage", "GET", "/api/v1/documents", reqData, nil)
	if err != nil {
		return nil, err
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("storage request failed with status %d", statusCode)
	}

	var result struct {
		Data []models.Role `json:"data"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	roles := make([]*models.Role, len(result.Data))
	for i := range result.Data {
		roles[i] = &result.Data[i]
	}

	return roles, nil
}

func (r *MongoRoleRepository) Update(ctx *saiTypes.RequestCtx, filter, data map[string]interface{}) error {
	reqData := map[string]interface{}{
		"collection": "roles",
		"filter":     filter,
		"data":       data,
	}

	_, statusCode, err := r.client.Call("storage", "PUT", "/api/v1/documents", reqData, nil)
	if err != nil {
		return err
	}

	if statusCode >= 400 {
		return fmt.Errorf("storage request failed with status %d", statusCode)
	}

	return nil
}

func (r *MongoRoleRepository) Delete(ctx *saiTypes.RequestCtx, filter map[string]interface{}) error {
	reqData := map[string]interface{}{
		"collection": "roles",
		"filter":     filter,
	}

	_, statusCode, err := r.client.Call("storage", "DELETE", "/api/v1/documents", reqData, nil)
	if err != nil {
		return err
	}

	if statusCode >= 400 {
		return fmt.Errorf("storage request failed with status %d", statusCode)
	}

	return nil
}

func (r *MongoRoleRepository) List(ctx *saiTypes.RequestCtx, filter *types.RoleFilterRequest) ([]*models.Role, int64, error) {
	mongoFilter := make(map[string]interface{})

	if filter.Search != "" {
		mongoFilter["name"] = map[string]interface{}{"$regex": filter.Search, "$options": "i"}
	}

	if filter.Active != nil {
		mongoFilter["is_active"] = *filter.Active
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 20
	}
	skip := (page - 1) * limit

	reqData := map[string]interface{}{
		"collection": "roles",
		"filter":     mongoFilter,
		"sort":       map[string]interface{}{"name": 1},
		"limit":      limit,
		"skip":       skip,
	}

	response, statusCode, err := r.client.Call("storage", "GET", "/api/v1/documents", reqData, nil)
	if err != nil {
		return nil, 0, err
	}

	if statusCode != 200 {
		return nil, 0, fmt.Errorf("storage request failed with status %d", statusCode)
	}

	var result struct {
		Data  []models.Role `json:"data"`
		Total int64         `json:"total"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, 0, err
	}

	roles := make([]*models.Role, len(result.Data))
	for i := range result.Data {
		roles[i] = &result.Data[i]
	}

	return roles, result.Total, nil
}

func (r *MongoRoleRepository) GetUsersByRole(ctx *saiTypes.RequestCtx, roleID string) ([]string, error) {
	reqData := map[string]interface{}{
		"collection": "users",
		"filter":     map[string]interface{}{"roles": map[string]interface{}{"$in": []string{roleID}}},
	}

	response, statusCode, err := r.client.Call("storage", "GET", "/api/v1/documents", reqData, nil)
	if err != nil {
		return nil, err
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("storage request failed with status %d", statusCode)
	}

	var result struct {
		Data []models.User `json:"data"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	userIDs := make([]string, len(result.Data))
	for i, user := range result.Data {
		userIDs[i] = user.InternalID
	}

	return userIDs, nil
}
