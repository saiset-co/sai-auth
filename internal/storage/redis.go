package storage

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/types"
	saiTypes "github.com/saiset-co/sai-service/types"
)

type RedisRateLimiter struct {
	client *redis.Client
}

func NewRedisRateLimiter(cfg types.RedisConfig) *RedisRateLimiter {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &RedisRateLimiter{client: client}
}

func (r *RedisRateLimiter) CheckRate(_ *saiTypes.RequestCtx, userID string, rate models.Rate) (bool, error) {
	key := fmt.Sprintf("rate_limit:%s:%d", userID, rate.Window)
	ctx := context.Background()

	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}

	if count == 1 {
		r.client.Expire(ctx, key, rate.Window)
	}

	return count <= rate.Limit, nil
}
