package api

import (
	"log/slog"
	"net/http"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

type Client struct {
	slack slack.Client
	teams []github.Team
}

func New(teams []github.Team, slack slack.Client) Client {
	return Client{
		slack: slack,
		teams: teams,
	}
}

func (c Client) Run(addr string) error {
	slog.Info("Starting server")
	http.HandleFunc("/events", c.eventsHandler)
	return http.ListenAndServe(addr, nil)
}
