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
		log.Error("creating Slack client", "err", err.Error())
		return
	}

	log.Info("Fetching users from Slack")
	users, err := slackAPI.ListUsers()
	if err != nil {
		log.Error("listing Slack users", "err", err.Error())
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
				log.Error("saving Slack user ID to database", "user", user.ID, "err", err.Error())
				return
			}
		}
	}
}
