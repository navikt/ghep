package ghep

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/navikt/ghep/internal/api"
	"github.com/navikt/ghep/internal/events"
	"github.com/navikt/ghep/internal/github"
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

	teams := make([]string, 0, len(teamConfig))
	for name := range teamConfig {
		teams = append(teams, name)
	}

	log.Info(fmt.Sprintf("Teams using Ghep: %s", strings.Join(teams, ", ")))

	log.Info("Ensuring Slack channels")
	if err := slackAPI.EnsureChannels(teamConfig); err != nil {
		return fmt.Errorf("ensuring Slack channels: %w", err)
	}

	log.Info("Creating event handler")
	eventHandler := events.NewHandler(githubClient, db, slackAPI, teamConfig)

	apiClient := api.New(
		log.With("client", "api"),
		db,
		eventHandler,
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
