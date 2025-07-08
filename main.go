package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/navikt/ghep/internal/api"
	"github.com/navikt/ghep/internal/events"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/redis"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func main() {
	ctx := context.Background()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	log.Info(fmt.Sprintf("Starting Ghep for %s", os.Getenv("GITHUB_ORG")))

	dbURL := os.Getenv("PGURL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/ghep"
	}

	db, err := sql.New(ctx, log.With("component", "db"), dbURL)
	if err != nil {
		log.Error("creating SQL client", "err", err.Error())
		os.Exit(1)
	}

	githubClient := github.New(
		log.With("component", "github"),
		db,
		os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		os.Getenv("GITHUB_APP_ID"),
		os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		os.Getenv("GITHUB_ORG"),
	)

	teamConfig, err := githubClient.ParseTeamConfig(ctx, os.Getenv("REPOS_CONFIG_FILE_PATH"))
	if err != nil {
		log.Error("parsing team config", "err", err.Error())
		os.Exit(1)
	}

	log.Info("Gettings repositories from Github")
	subscribeToOrg, _ := strconv.ParseBool(os.Getenv("GHEP_SUBSCRIBE_TO_ORG"))
	if err := githubClient.FetchTeams(
		ctx,
		os.Getenv("GITHUB_BLOCKLIST_REPOS"),
		os.Getenv("GITHUB_ORG"),
		subscribeToOrg,
	); err != nil {
		log.Error("fetching teams from Github", "err", err.Error())
		os.Exit(1)
	}

	logTeams(ctx, log, db)

	log.Info("Creating Slack client")
	slackAPI, err := slack.New(
		log.With("component", "slack"),
		os.Getenv("SLACK_TOKEN"),
	)
	if err != nil {
		log.Error("creating Slack client", "err", err.Error())
		os.Exit(1)
	}

	log.Info("Ensuring Slack channels")
	if err := slackAPI.EnsureChannels(teamConfig); err != nil {
		log.Error("ensuring Slack channels", "err", err.Error())
		os.Exit(1)
	}

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
	eventHandler := events.NewHandler(githubClient, rdb, db, slackAPI, teamConfig)

	if err := githubClient.FetchOrgMembers(ctx); err != nil {
		log.Error("fetching org members", "err", err.Error())
	}

	apiClient := api.New(
		log.With("component", "api"),
		db,
		eventHandler,
		rdb,
		teamConfig,
		os.Getenv("EXTERNAL_CONTRIBUTORS_CHANNEL"),
		subscribeToOrg,
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

func logTeams(ctx context.Context, log *slog.Logger, db *gensql.Queries) {
	teams, err := db.ListTeams(ctx)
	if err != nil {
		log.Error("listing teams from database", "err", err.Error())
		return
	}

	log.Info(fmt.Sprintf("Teams using Ghep: %v", teams))
}
