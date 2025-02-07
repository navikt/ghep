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
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	log.Info(fmt.Sprintf("Starting Ghep for %s", os.Getenv("GITHUB_ORG")))

	githubClient := github.New(
		log.With("component", "github"),
		os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		os.Getenv("GITHUB_APP_ID"),
		os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		os.Getenv("GITHUB_ORG"),
	)

	log.Info("Gettings repositories from Github")
	teams, err := githubClient.FetchTeams(
		os.Getenv("REPOS_CONFIG_FILE_PATH"),
		os.Getenv("GITHUB_BLOCKLIST_REPOS"),
		os.Getenv("GHEP_SUBSCRIBE_TO_ORG"),
	)
	if err != nil {
		log.Error("fetching teams from Github", "err", err.Error())
		os.Exit(1)
	}

	logTeams(log, teams)

	log.Info("Creating Slack client")
	slackApi, err := slack.New(
		log.With("component", "slack"),
		os.Getenv("SLACK_TOKEN"),
	)
	if err != nil {
		log.Error("creating Slack client", "err", err.Error())
		os.Exit(1)
	}

	log.Info("Ensuring Slack channels")
	if err := slackApi.EnsureChannels(teams); err != nil {
		log.Error("ensuring Slack channels", "err", err.Error())
		os.Exit(1)
	}

	ctx := context.Background()

	log.Info("Creating Redis client")
	rdb, err := redis.New(
		ctx,
		log.With("component", "redis"),
		os.Getenv("REDIS_URI_EVENTS"),
		os.Getenv("REDIS_USERNAME_EVENTS"),
		os.Getenv("REDIS_PASSWORD_EVENTS"),
	)
	if err != nil {
		log.Error("creating Redis client", "err", err.Error())
		os.Exit(1)
	}

	log.Info("Creating event handler")
	eventHandler := events.NewHandler(githubClient, rdb, slackApi, teams)

	orgMembers, err := githubClient.FetchOrgMembers()
	if err != nil {
		log.Error("fetching org members", "err", err.Error())
	}

	apiClient := api.New(
		log.With("component", "api"),
		eventHandler,
		rdb,
		teams,
		orgMembers,
		os.Getenv("EXTERNAL_CONTRIBUTORS_CHANNEL"),
	)

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = "0.0.0.0:8080"
	}

	log.Info("Starting API server")
	if err := apiClient.Run(os.Getenv("API_BASE_PATH"), addr); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func logTeams(log *slog.Logger, teams []github.Team) {
	var teamNames []string
	for _, team := range teams {
		teamNames = append(teamNames, team.Name)
	}

	log.Info(fmt.Sprintf("Teams using Ghep: %v", teamNames))
}
