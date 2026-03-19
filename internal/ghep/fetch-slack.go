package ghep

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func FetchSlackUsers(ctx context.Context, log *slog.Logger, db *gensql.Queries) {
	log.Info("Creating Slack client")
	slackAPI, err := slack.New(
		log.With("client", "slack"),
		os.Getenv("SLACK_TOKEN"),
	)
	if err != nil {
		log.Error("Creating Slack client", "error", err)
		return
	}

	log.Info("Fetching users from Slack")
	users, err := slackAPI.ListUsers()
	if err != nil {
		log.Error("Listing Slack users", "error", err)
		return
	}

	log.Info("Saving Slack users ID to database")
	for _, user := range users {
		login, err := db.GetUserByEmail(ctx, user.Email)
		if err != nil && errors.Is(err, pgx.ErrNoRows) {
			continue
		}

		if login != "" {
			if err := db.CreateSlackID(ctx, gensql.CreateSlackIDParams{
				Login: login,
				ID:    user.ID,
			}); err != nil {
				log.Error("Saving Slack user ID to database", "user", user.ID, "error", err)
				return
			}
		}
	}
}
