package events

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func (h *Handler) handleDependabotAlertEvent(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	if team.SlackChannels.Security == "" {
		return nil, nil
	}

	if !slices.Contains([]string{"created", "fixed", "dismissed", "reintroduced", "reopened"}, event.Action) {
		return nil, nil
	}

	if team.Config.Security.IgnoreThreshold(event.Alert.SecurityAdvisory.Severity) {
		return nil, nil
	}

	var timestamp string
	channel := team.SlackChannels.Security
	// TODO: Burde ha bedre kontroll på hvilke Actions vi håndterer
	if event.Action != "created" {
		message, err := h.db.GetSlackMessage(ctx, gensql.GetSlackMessageParams{
			TeamSlug: team.Name,
			EventID:  event.Alert.URL,
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", event.Alert.URL)
		}

		timestamp = message.ThreadTs
		if timestamp != "" {
			reaction := slack.ReactionDefault
			switch event.Action {
			case "dismissed", "auto_dismissed":
				reaction = slack.ReactionCancelled
			case "fixed":
				reaction = slack.ReactionSuccess
			}

			log.Info("Posting reaction to Dependabot alert", "action", event.Action, "alert_state", event.Alert.State, "timestamp", timestamp, "reaction", reaction)
			if err := h.slack.PostReaction(team.SlackChannels.Workflows, timestamp, reaction); err != nil {
				log.Error("error posting reaction", "err", err.Error(), "channel", team.SlackChannels.Workflows, "timestamp", timestamp, "reaction", reaction)
			}
		}

		if message.Channel != "" {
			channel = message.Channel
		}
	}

	log.Info("Received Dependabot alert event")
	return slack.CreateDependabotAlertMessage(channel, event, timestamp), nil
}
