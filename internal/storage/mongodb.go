package storage

import (
	"fmt"

	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/internal/repository"
	"github.com/saiset-co/sai-auth/types"
	"github.com/saiset-co/sai-service/sai"
	saiTypes "github.com/saiset-co/sai-service/types"
)

type MongoUserRepository struct {
	client saiTypes.ClientManager
}

func NewMongoUserRepository() repository.UserRepository {
	return &MongoUserRepository{
		client: sai.ClientManager(),
	}
}

func (r *MongoUserRepository) Create(ctx *saiTypes.RequestCtx, user *models.User) error {
	reqData := map[string]interface{}{
		"collection": "users",
		"data":       []interface{}{user},
	}

	_, _, err := r.client.Call("storage", "POST", "/api/v1/documents", reqData, nil)
	return err
}

func (r *MongoUserRepository) GetByID(ctx *saiTypes.RequestCtx, id string) (*models.User, error) {
	reqData := map[string]interface{}{
		"collection": "users",
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
		Data []models.User `json:"data"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return &result.Data[0], nil
}

func (r *MongoUserRepository) GetByUsername(ctx *saiTypes.RequestCtx, username string) (*models.User, error) {
	reqData := map[string]interface{}{
		"collection": "users",
		"filter":     map[string]interface{}{"username": username},
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
		Data []models.User `json:"data"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return &result.Data[0], nil
}

func (r *MongoUserRepository) GetByEmail(ctx *saiTypes.RequestCtx, email string) (*models.User, error) {
	reqData := map[string]interface{}{
		"collection": "users",
		"filter":     map[string]interface{}{"email": email},
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
		Data []models.User `json:"data"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return &result.Data[0], nil
}

func (r *MongoUserRepository) Update(ctx *saiTypes.RequestCtx, filter, data map[string]interface{}) error {
	reqData := map[string]interface{}{
		"collection": "users",
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

func (r *MongoUserRepository) Delete(ctx *saiTypes.RequestCtx, filter map[string]interface{}) error {
	reqData := map[string]interface{}{
		"collection": "users",
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

func (r *MongoUserRepository) List(ctx *saiTypes.RequestCtx, filter *types.UserFilterRequest) ([]*models.User, int64, error) {
	mongoFilter := make(map[string]interface{})

	if filter.Search != "" {
		mongoFilter["$or"] = []interface{}{
			map[string]interface{}{"username": map[string]interface{}{"$regex": filter.Search, "$options": "i"}},
			map[string]interface{}{"email": map[string]interface{}{"$regex": filter.Search, "$options": "i"}},
		}
	}

	if filter.Role != "" {
		mongoFilter["roles"] = map[string]interface{}{"$in": []string{filter.Role}}
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
		"collection": "users",
		"filter":     mongoFilter,
		"sort":       map[string]interface{}{"username": 1},
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
		Data  []models.User `json:"data"`
		Total int64         `json:"total"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, 0, err
	}

	users := make([]*models.User, len(result.Data))
	for i := range result.Data {
		users[i] = &result.Data[i]
	}

	return users, result.Total, nil
}

func (r *MongoUserRepository) GetFirstUser(ctx *saiTypes.RequestCtx) (*models.User, error) {
	reqData := map[string]interface{}{
		"collection": "users",
		"sort":       map[string]interface{}{"cr_time": 1},
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
		Data []models.User `json:"data"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no users found")
	}

	return &result.Data[0], nil
}

func (r *MongoUserRepository) CountUsers(ctx *saiTypes.RequestCtx) (int64, error) {
	reqData := map[string]interface{}{
		"collection": "users",
		"filter":     map[string]interface{}{},
		"limit":      1,
	}

	response, statusCode, err := r.client.Call("storage", "GET", "/api/v1/documents", reqData, nil)
	if err != nil {
		return 0, err
	}

	if statusCode != 200 {
		return 0, fmt.Errorf("storage request failed with status %d", statusCode)
	}

	var result struct {
		Total int64 `json:"total"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return 0, err
	}

	return result.Total, nil
}
