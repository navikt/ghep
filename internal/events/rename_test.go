package events

import (
	"context"
	"log/slog"
	"testing"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/mock"
	"github.com/navikt/ghep/internal/testdata"
)

func TestHandleRename(t *testing.T) {
	db := &mock.Database{Members: []string{}}
	slack := &mock.Slack{}
	team := github.Team{
		Name:          "test",
		SlackChannels: github.SlackChannels{},
		Config:        github.Config{},
		Sources: []github.Source{
			{
				SourceType: "commits",
				Channel:    "#test",
			},
		},
	}
	handler := NewHandler(db, slack, map[string]github.Team{"test": team})

	t.Run("", func(t *testing.T) {
		event, err := testdata.AsEvent("renamed-1.json")
		if err != nil {
			t.Error(err)
		}

		if err := handler.handleSource(
			context.TODO(),
			slog.Default(),
			team,
			team.Sources[0],
			event,
		); err != nil {
			t.Error(err)
		}

		if len(slack.Messages) != 1 {
			t.Errorf("handleRenameEvent() did not send just one message: %d", len(slack.Messages))
		}
	})
}
