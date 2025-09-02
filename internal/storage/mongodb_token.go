package storage

import (
	"fmt"
	"time"

	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/internal/repository"
	"github.com/saiset-co/sai-auth/types"
	"github.com/saiset-co/sai-service/sai"
	saiTypes "github.com/saiset-co/sai-service/types"
)

type MongoTokenRepository struct {
	client saiTypes.ClientManager
}

func NewMongoTokenRepository() repository.TokenRepository {
	return &MongoTokenRepository{
		client: sai.ClientManager(),
	}
}

func (r *MongoTokenRepository) Store(ctx *saiTypes.RequestCtx, token *models.Token) error {
	reqData := map[string]interface{}{
		"collection": "tokens",
		"data":       []interface{}{token},
	}

	_, _, err := r.client.Call("storage", "POST", "/api/v1/documents", reqData, nil)
	return err
}

func (r *MongoTokenRepository) GetByAccessToken(ctx *saiTypes.RequestCtx, accessToken string) (*models.Token, error) {
	reqData := map[string]interface{}{
		"collection": "tokens",
		"filter":     map[string]interface{}{"access_token": accessToken},
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
		Data []models.Token `json:"data"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("token not found")
	}

	token := &result.Data[0]

	if time.Now().UnixNano() > token.ExpiresAt {
		r.Delete(ctx, token.InternalID)
		return nil, fmt.Errorf("token expired")
	}

	return token, nil
}

func (r *MongoTokenRepository) GetByRefreshToken(ctx *saiTypes.RequestCtx, refreshToken string) (*models.Token, error) {
	reqData := map[string]interface{}{
		"collection": "tokens",
		"filter":     map[string]interface{}{"refresh_token": refreshToken},
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
		Data []models.Token `json:"data"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("refresh token not found")
	}

	token := &result.Data[0]

	if time.Now().UnixNano() > token.RefreshExpiresAt {
		r.Delete(ctx, token.InternalID)
		return nil, fmt.Errorf("refresh token expired")
	}

	return token, nil
}

func (r *MongoTokenRepository) GetByUserID(ctx *saiTypes.RequestCtx, userID string) (*models.Token, error) {
	reqData := map[string]interface{}{
		"collection": "tokens",
		"filter":     map[string]interface{}{"user_id": userID},
		"limit":      1,
		"sort":       map[string]interface{}{"cr_time": -1},
	}

	response, statusCode, err := r.client.Call("storage", "GET", "/api/v1/documents", reqData, nil)
	if err != nil {
		return nil, err
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("storage request failed with status %d", statusCode)
	}

	var result struct {
		Data []models.Token `json:"data"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("user token not found")
	}

	return &result.Data[0], nil
}

func (r *MongoTokenRepository) Update(ctx *saiTypes.RequestCtx, token *models.Token) error {
	filter := map[string]interface{}{
		"internal_id": token.InternalID,
	}

	updateData := map[string]interface{}{
		"$set": map[string]interface{}{
			"access_token":       token.AccessToken,
			"refresh_token":      token.RefreshToken,
			"expires_at":         token.ExpiresAt,
			"refresh_expires_at": token.RefreshExpiresAt,
		},
	}

	reqData := map[string]interface{}{
		"collection": "tokens",
		"filter":     filter,
		"data":       updateData,
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

func (r *MongoTokenRepository) Delete(ctx *saiTypes.RequestCtx, tokenID string) error {
	filter := map[string]interface{}{
		"internal_id": tokenID,
	}

	reqData := map[string]interface{}{
		"collection": "tokens",
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

func (r *MongoTokenRepository) DeleteByUserID(ctx *saiTypes.RequestCtx, userID string) error {
	filter := map[string]interface{}{
		"user_id": userID,
	}

	reqData := map[string]interface{}{
		"collection": "tokens",
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

func (r *MongoTokenRepository) List(ctx *saiTypes.RequestCtx, filter *types.TokenFilterRequest) ([]*models.Token, int64, error) {
	mongoFilter := make(map[string]interface{})

	if filter.UserID != "" {
		mongoFilter["user_id"] = filter.UserID
	}

	if filter.Active != nil {
		if *filter.Active {
			mongoFilter["expires_at"] = map[string]interface{}{
				"$gt": time.Now().UnixNano(),
			}
		} else {
			mongoFilter["expires_at"] = map[string]interface{}{
				"$lte": time.Now().UnixNano(),
			}
		}
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
		"collection": "tokens",
		"filter":     mongoFilter,
		"limit":      limit,
		"skip":       skip,
		"sort":       map[string]interface{}{"cr_time": -1},
	}

	response, statusCode, err := r.client.Call("storage", "GET", "/api/v1/documents", reqData, nil)
	if err != nil {
		return nil, 0, err
	}

	if statusCode != 200 {
		return nil, 0, fmt.Errorf("storage request failed with status %d", statusCode)
	}

	var result struct {
		Data  []models.Token `json:"data"`
		Total int64          `json:"total"`
	}

	if err := ctx.Unmarshal(response, &result); err != nil {
		return nil, 0, err
	}

	tokens := make([]*models.Token, len(result.Data))
	for i := range result.Data {
		tokens[i] = &result.Data[i]
	}

	return tokens, result.Total, nil
}

func (r *MongoTokenRepository) IsValid(ctx *saiTypes.RequestCtx, accessToken string) bool {
	token, err := r.GetByAccessToken(ctx, accessToken)
	return err == nil && token != nil
}
