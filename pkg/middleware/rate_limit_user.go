package middleware

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"

	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/internal/storage"
	"github.com/saiset-co/sai-auth/types"

	"github.com/saiset-co/sai-service/sai"
	saiTypes "github.com/saiset-co/sai-service/types"
	"go.uber.org/zap"
)

type RateLimitUserMiddleware struct {
	rateLimiter *storage.RedisRateLimiter
	config      saiTypes.ConfigManager
	logger      saiTypes.Logger
}

func NewRateLimitUserMiddleware(redisConfig types.RedisConfig) *RateLimitUserMiddleware {
	return &RateLimitUserMiddleware{
		rateLimiter: storage.NewRedisRateLimiter(redisConfig),
		config:      sai.Config(),
		logger:      sai.Logger(),
	}
}

func (m *RateLimitUserMiddleware) Name() string {
	return "rate_limit_user"
}

func (m *RateLimitUserMiddleware) Weight() int {
	return 80
}

func (m *RateLimitUserMiddleware) Handle(ctx *saiTypes.RequestCtx, next func(*saiTypes.RequestCtx), _ *saiTypes.RouteConfig) {
	userID := ctx.UserValue("user_id")
	if userID == nil {
		next(ctx)
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		next(ctx)
		return
	}

	modifiedParams := ctx.UserValue("auth_modified_params")
	if modifiedParams == nil {
		next(ctx)
		return
	}

	params, ok := modifiedParams.(map[string]interface{})
	if !ok {
		next(ctx)
		return
	}

	rates := m.extractRatesFromParams(params)
	if len(rates) == 0 {
		next(ctx)
		return
	}

	for _, rate := range rates {
		allowed, err := m.rateLimiter.CheckRate(ctx, userIDStr, rate)
		if err != nil {
			m.logger.Error("Rate limit check failed", zap.Error(err), zap.String("user_id", userIDStr))
			next(ctx)
			return
		}

		if !allowed {
			ctx.Error(errors.New("Rate limit exceeded"), fasthttp.StatusTooManyRequests)
			return
		}
	}

	next(ctx)
}

func (m *RateLimitUserMiddleware) extractRatesFromParams(params map[string]interface{}) []models.Rate {
	ratesInterface, exists := params["rates"]
	if !exists {
		return nil
	}

	ratesData, err := json.Marshal(ratesInterface)
	if err != nil {
		return nil
	}

	var rates []models.Rate
	err = json.Unmarshal(ratesData, &rates)
	if err != nil {
		return nil
	}

	return rates
}
