package providers

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/saiset-co/sai-service/sai"
	"github.com/saiset-co/sai-service/types"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type SaiAuthProvider struct {
	name           string
	authServiceURL string
	timeout        time.Duration
	cachedToken    string
	tokenExpiry    time.Time
}

func NewSaiAuthProvider(name, authServiceURL string) *SaiAuthProvider {
	return &SaiAuthProvider{
		name:           name,
		authServiceURL: authServiceURL,
		timeout:        30 * time.Second,
	}
}

func (p *SaiAuthProvider) Type() string {
	return "sai_auth"
}

func (p *SaiAuthProvider) ApplyToIncomingRequest(ctx *types.RequestCtx) error {
	requestData := map[string]interface{}{
		"token":          p.extractToken(ctx),
		"microservice":   p.name,
		"method":         string(ctx.Method()),
		"path":           string(ctx.Path()),
		"request_params": p.extractRequestParams(ctx),
	}

	allowed, userID, modifiedParams, err := p.verifyWithAuthService(requestData)
	if err != nil {
		sai.Logger().Error("SaiAuthProvider: Verification failed", zap.Error(err))
		return err
	}

	if !allowed {
		sai.Logger().Warn("SaiAuthProvider: Access denied",
			zap.String("microservice", p.name),
			zap.String("method", string(ctx.Method())),
			zap.String("path", string(ctx.Path())),
			zap.Any("request_params", requestData["request_params"]))
		return errors.New("access denied")
	}

	ctx.SetUserValue("user_id", userID)

	if modifiedParams != nil {
		p.applyModifiedParams(ctx, modifiedParams)
	}

	return nil
}

func (p *SaiAuthProvider) extractToken(ctx *types.RequestCtx) string {
	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	if strings.HasPrefix(authHeader, "Token ") {
		return strings.TrimPrefix(authHeader, "Token ")
	}
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

func (p *SaiAuthProvider) extractRequestParams(ctx *types.RequestCtx) map[string]interface{} {
	body := ctx.PostBody()
	if len(body) > 0 {
		var params map[string]interface{}
		json.Unmarshal(body, &params)
		return params
	}

	args := ctx.QueryArgs()
	params := make(map[string]interface{})
	args.VisitAll(func(key, value []byte) {
		params[string(key)] = string(value)
	})

	return params
}

func (p *SaiAuthProvider) verifyWithAuthService(requestData map[string]interface{}) (bool, string, map[string]interface{}, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	reqBody, _ := json.Marshal(requestData)
	req.SetRequestURI(p.authServiceURL + "/api/v1/auth/verify")
	req.Header.SetMethod("POST")
	req.Header.SetContentType("application/json")
	req.SetBody(reqBody)

	err := fasthttp.DoTimeout(req, resp, p.timeout)
	if err != nil {
		sai.Logger().Error("SaiAuthProvider request failed", zap.Error(err))
		return false, "", nil, err
	}

	if resp.StatusCode() == 200 {
		var result struct {
			Allowed        bool                   `json:"allowed"`
			UserID         string                 `json:"user_id"`
			ModifiedParams map[string]interface{} `json:"modified_params"`
		}

		err = json.Unmarshal(resp.Body(), &result)
		return result.Allowed, result.UserID, result.ModifiedParams, err
	}

	return false, "", nil, errors.New("authorization failed")
}

func (p *SaiAuthProvider) applyModifiedParams(ctx *types.RequestCtx, params map[string]interface{}) {
	ctx.SetUserValue("auth_modified_params", params)
}

func (p *SaiAuthProvider) getToken(authConfig *types.ServiceAuthConfig) (string, error) {
	// Проверяем кэшированный токен
	if p.cachedToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.cachedToken, nil
	}

	// Получаем новый токен через аутентификацию
	username, ok := authConfig.Payload["username"].(string)
	if !ok {
		return "", errors.New("username not found in auth payload")
	}

	password, ok := authConfig.Payload["password"].(string)
	if !ok {
		return "", errors.New("password not found in auth payload")
	}

	token, err := p.authenticateAndGetToken(username, password)
	if err != nil {
		return "", err
	}

	// Кэшируем токен на 1 час
	p.cachedToken = token
	p.tokenExpiry = time.Now().Add(1 * time.Hour)

	return token, nil
}

func (p *SaiAuthProvider) authenticateAndGetToken(username, password string) (string, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	loginData := map[string]interface{}{
		"user":     username,
		"password": password,
	}

	reqBody, _ := json.Marshal(loginData)
	req.SetRequestURI(p.authServiceURL + "/api/v1/auth/login")
	req.Header.SetMethod("POST")
	req.Header.SetContentType("application/json")
	req.SetBody(reqBody)

	err := fasthttp.DoTimeout(req, resp, p.timeout)
	if err != nil {
		sai.Logger().Error("SaiAuthProvider login failed", zap.Error(err))
		return "", err
	}

	if resp.StatusCode() != 200 {
		sai.Logger().Error("SaiAuthProvider login failed", 
			zap.Int("status", resp.StatusCode()),
			zap.String("response", string(resp.Body())))
		return "", errors.New("authentication failed")
	}

	var result struct {
		Tokens struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
	}

	err = json.Unmarshal(resp.Body(), &result)
	if err != nil {
		return "", err
	}

	if result.Tokens.AccessToken == "" {
		return "", errors.New("no access token in response")
	}

	return result.Tokens.AccessToken, nil
}

func (p *SaiAuthProvider) ApplyToOutgoingRequest(req *fasthttp.Request, authConfig *types.ServiceAuthConfig) error {
	if authConfig == nil || authConfig.Payload == nil {
		return errors.New("auth config required")
	}

	token, err := p.getToken(authConfig)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Token "+token)
	return nil
}
