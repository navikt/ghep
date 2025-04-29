package events

import (
	"context"
	"log/slog"
	"slices"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

func (h *Handler) handleTeamEvent(ctx context.Context, log *slog.Logger, event github.Event) (*slack.Message, error) {
	index := slices.IndexFunc(h.teams, func(t *github.Team) bool {
		return t.Name == event.Team.Name
	})

	if index == -1 {
		return nil, nil
	}

	team := h.teams[index]

	payload, err := handleTeamEvent(log, team, event)
	h.teams[index] = team

	return payload, err
}

func handleTeamEvent(log *slog.Logger, team *github.Team, event github.Event) (*slack.Message, error) {
	if !slices.Contains([]string{"added_to_repository", "removed_from_repository", "added", "removed"}, event.Action) {
		return nil, nil
	}

	log.Info("Received team event", "triggered_by", event.Sender.Login)
	switch event.Action {
	case "added_to_repository":
		team.AddRepository(event.Repository.Name)
	case "removed_from_repository":
		team.RemoveRepository(event.Repository.Name)
	case "added":
		team.AddMember(event.Member)
	case "removed":
		team.RemoveMember(event.Member.Login)
	}

	if team.SlackChannels.Commits == "" {
		return nil, nil
	}

	return slack.CreateTeamMessage(team.SlackChannels.Commits, event), nil
}
