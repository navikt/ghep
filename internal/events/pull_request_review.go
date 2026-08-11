package events

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func (h *Handler) handlePullRequestReviewEvent(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	if !slices.Contains([]string{"approved", "changes_requested"}, event.Review.State) {
		return nil, nil
	}

	pullRequests, err := h.db.ListSlackMessagesByEvent(ctx, gensql.ListSlackMessagesByEventParams{
		TeamSlug: team.Name,
		EventID:  strconv.Itoa(event.PullRequest.ID),
	})
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	for _, pullRequest := range pullRequests {
		reaction := slack.ReactionDefault
		switch event.Review.State {
		case "approved":
			reaction = slack.ReactionApproved
		case "changes_requested":
			reaction = slack.ReactionRequest
		}

		log.Info("Reacting to reviewed pull request", "action", event.Action, "review_state", event.Review.State, "reaction", reaction)
		if err := h.slack.PostReaction(pullRequest.Channel, pullRequest.ThreadTs, reaction); err != nil {
			log.Error("Posting pull request reaction", "error", err, "channel", pullRequest.Channel, "timestamp", pullRequest.ThreadTs)
		}
	}

	return nil, nil
}
