package redis

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

func New(ctx context.Context, log *slog.Logger, redisAddr, redisUsername, redisPassword string) (*redis.Client, error) {
	opt, err := redis.ParseURL(redisAddr)
	if err != nil {
		return nil, fmt.Errorf("parsing redis URL: %w", err)
	}

	opt.Username = redisUsername
	opt.Password = redisPassword

	rdb := redis.NewClient(opt)

	rsl, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("pinging redis: %w", err)
	}

	log.Info("Redis connection established", "response", rsl)

	return rdb, nil
}
