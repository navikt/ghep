package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	slack slack.Client
	teams []github.Team
	rdb   *redis.Client
	ctx   context.Context
}

func New(ctx context.Context, teams []github.Team, slack slack.Client, redisAddr, redisUsername, redisPassword string) (Client, error) {
	opt, err := redis.ParseURL(redisAddr)
	if err != nil {
		return Client{}, err
	}

	opt.Username = redisUsername
	opt.Password = redisPassword

	rdb := redis.NewClient(opt)

	rsl, err := rdb.Ping(ctx).Result()
	if err != nil {
		return Client{}, err
	}
	slog.Info("Redis connection established", "response", rsl)

	return Client{
		slack: slack,
		teams: teams,
		rdb:   rdb,
		ctx:   ctx,
	}, nil
}

func (c Client) Run(addr string) error {
	slog.Info("Starting server")
	http.HandleFunc("/events", c.eventsHandler)
	return http.ListenAndServe(addr, nil)
}
