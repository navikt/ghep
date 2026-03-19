package events

import (
	"context"
	"log/slog"
	"slices"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql"
	"github.com/navikt/ghep/internal/sql/gensql"
)

// handleTeamSideEffects performs DB operations for team events (add/remove repos/members).
func (h *Handler) handleTeamSideEffects(ctx context.Context, log *slog.Logger, event github.Event) error {
	if !slices.Contains([]string{"added_to_repository", "removed_from_repository", "added", "removed"}, event.Action) {
		return nil
	}

	team := event.Team.Name
	log.Info("Received team event", "triggered_by", event.Sender.Login)

	switch event.Action {
	case "added_to_repository":
		if err := sql.AddRepositoryToTeam(ctx, h.db, team, event.Repository.Name); err != nil {
			return err
		}
	case "removed_from_repository":
		if err := h.db.RemoveTeamRepository(ctx, gensql.RemoveTeamRepositoryParams{
			TeamSlug: team,
			Name:     event.Repository.Name,
		}); err != nil {
			return err
		}
	case "added":
		if err := sql.AddMemberToTeam(ctx, h.db, team, event.Member.Login); err != nil {
			return err
		}
	case "removed":
		if err := h.db.RemoveTeamMember(ctx, gensql.RemoveTeamMemberParams{
			TeamSlug:  team,
			UserLogin: event.Member.Login,
		}); err != nil {
			return err
		}
	}

	return nil
}

func handleTeamEvent(_ *slog.Logger, channel string, event github.Event) (*slack.Message, error) {
	if channel == "" {
		return nil, nil
	}

	if !slices.Contains([]string{"added_to_repository", "removed_from_repository", "added", "removed"}, event.Action) {
		return nil, nil
	}

	return slack.CreateTeamMessage(channel, event), nil
}
