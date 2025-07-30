package ghep

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
	"github.com/navikt/ghep/internal/sql/gensql"
)

func Run(ctx context.Context, log *slog.Logger, db *gensql.Queries, teamConfig map[string]github.Team, githubClient github.Client, subscribeToOrg bool) error {
	log.Info(fmt.Sprintf("Starting Ghep for %s", os.Getenv("GITHUB_ORG")))

	log.Info("Creating Slack client")
	slackAPI, err := slack.New(
		log.With("client", "slack"),
		os.Getenv("SLACK_TOKEN"),
	)
	if err != nil {
		return fmt.Errorf("creating Slack client: %w", err)
	}

	logTeams(ctx, log, db)

	log.Info("Ensuring Slack channels")
	if err := slackAPI.EnsureChannels(teamConfig); err != nil {
		return fmt.Errorf("ensuring Slack channels: %w", err)
	}

	log.Info("Creating Redis client")
	rdb, err := redis.New(
		ctx,
		log.With("client", "redis"),
		os.Getenv("REDIS_URI_EVENTS"),
		os.Getenv("REDIS_USERNAME_EVENTS"),
		os.Getenv("REDIS_PASSWORD_EVENTS"),
	)
	if err != nil {
		return fmt.Errorf("creating Redis client: %w", err)
	}

	log.Info("Creating event handler")
	eventHandler := events.NewHandler(githubClient, rdb, db, slackAPI, teamConfig)

	if err := githubClient.FetchOrgMembers(ctx); err != nil {
		return fmt.Errorf("fetching org members: %w", err)
	}

	apiClient := api.New(
		log.With("client", "api"),
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
		return fmt.Errorf("starting API server: %w", err)
	}

	return nil
}

func logTeams(ctx context.Context, log *slog.Logger, db *gensql.Queries) {
	teams, err := db.ListTeams(ctx)
	if err != nil {
		log.Error("listing teams from database", "err", err.Error())
		return
	}

	log.Info(fmt.Sprintf("Teams using Ghep: %v", teams))
}
