package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

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

func New(ctx context.Context, teams []github.Team, slack slack.Client, redisAddr, redisUsername, redisPassword string) Client {
	redisAddr = strings.TrimPrefix(redisAddr, "rediss://")
	return Client{
		slack: slack,
		teams: teams,
		rdb: redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Username: redisUsername,
			Password: redisPassword,
		}),
		ctx: ctx,
	}
}

func (c Client) Run(addr string) error {
	slog.Info("Starting server")
	http.HandleFunc("/events", c.eventsHandler)
	return http.ListenAndServe(addr, nil)
}
