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
		h.slack.PostPullRequestReaction(log, event.Review.State, pullRequest.Channel, pullRequest.ThreadTs)
	}

	return nil, nil
}
