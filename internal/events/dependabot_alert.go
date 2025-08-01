package events

import (
	"context"
	"errors"
	"log/slog"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/redis/go-redis/v9"
)

func (h *Handler) handleDependabotAlertEvent(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	if team.SlackChannels.Security == "" {
		return nil, nil
	}

	if team.Config.Security.IgnoreThreshold(event.Alert.SecurityAdvisory.Severity) {
		return nil, nil
	}

	var timestamp string
	if event.Action != "created" {
		var err error
		timestamp, err = h.redis.Get(ctx, event.Alert.URL).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", event.Alert.URL)

			return nil, nil
		}

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
				log.Error("error posting reaction", "err", err.Error(), "channel", team.SlackChannels.Security, "timestamp", timestamp, "reaction", reaction)
			}
		}
	}

	log.Info("Received Dependabot alert event")
	return slack.CreateDependabotAlertMessage(team.SlackChannels.Security, event, timestamp), nil
}
