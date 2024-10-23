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

func New(ctx context.Context, teams []github.Team, slack slack.Client, rdb *redis.Client) (Client, error) {
	return Client{
		ctx:   ctx,
		slack: slack,
		teams: teams,
		rdb:   rdb,
	}, nil
}

func (c Client) Run(addr string) error {
	slog.Info("Starting server")
	http.HandleFunc("POST /events", c.eventsPostHandler)
	http.HandleFunc("GET /internal/health", c.healthGetHandler)
	return http.ListenAndServe(addr, nil)
}
