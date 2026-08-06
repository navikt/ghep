package events

import (
	"context"
	"log/slog"
	"strings"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql"
)

func handleCommitEvent(ctx context.Context, log *slog.Logger, source github.Source, event github.Event, db sql.Database) (*slack.Message, error) {
	branch := strings.TrimPrefix(event.Ref, github.RefHeadsPrefix)

	if len(source.Config.Branches) == 0 && branch != event.Repository.DefaultBranch {
		return nil, nil
	}

	if len(event.Commits) == 0 {
		return nil, nil
	}

	log = log.With("channel", source.Channel)
	log.Info("Received commit event")
	return slack.CreateCommitMessage(ctx, log, db, source.Channel, event)
}
