package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/navikt/ghep/internal/api"
	"github.com/navikt/ghep/internal/events"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/redis"
	"github.com/navikt/ghep/internal/slack"
)

func main() {
	githubClient := github.New(
		os.Getenv("GITHUB_API"),
		os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		os.Getenv("GITHUB_APP_ID"),
		os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		os.Getenv("GITHUB_ORG"),
	)

	teams, err := githubClient.FetchTeams(
		os.Getenv("REPOS_CONFIG_FILE_PATH"),
		os.Getenv("GITHUB_BLOCKLIST_REPOS"),
	)
	if err != nil {
		slog.Error("fetching teams from Github", "err", err.Error())
		os.Exit(1)
	}

	logTeams(teams)

	slackApi, err := slack.New(os.Getenv("SLACK_TOKEN"))
	if err != nil {
		slog.Error("creating Slack client", "err", err.Error())
		os.Exit(1)
	}

	ctx := context.Background()

	rdb, err := redis.New(
		ctx,
		os.Getenv("REDIS_URI_EVENTS"),
		os.Getenv("REDIS_USERNAME_EVENTS"),
		os.Getenv("REDIS_PASSWORD_EVENTS"),
	)
	if err != nil {
		slog.Error("creating Redis client", "err", err.Error())
		os.Exit(1)
	}

	eventHandler := events.NewHandler(githubClient, rdb, slackApi, teams)

	api := api.New(
		eventHandler,
		rdb,
		teams,
	)

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = "0.0.0.0:8080"
	}

	if err := api.Run(addr); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func logTeams(teams []github.Team) {
	teamNames := []string{}
	for _, team := range teams {
		teamNames = append(teamNames, team.Name)
	}

	slog.Info(fmt.Sprintf("Teams using Ghep: %v", teamNames))
}
