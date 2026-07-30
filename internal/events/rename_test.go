package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/mock"
)

func TestHandleRename(t *testing.T) {
	db := &mock.Database{Members: []string{}}
	slack := &mock.Slack{}
	teams := map[string]github.Team{
		"test": {
			Name:          "test",
			SlackChannels: github.SlackChannels{},
			Config:        github.Config{},
			Sources: []github.Source{
				{
					SourceType: "commits",
					Channel:    "#test",
				},
			},
		},
	}

	handler := NewHandler(db, slack, teams)
	team := teams["test"]

	goldenfilePath := filepath.Join("../testdata/events", "renamed-1.json")
	goldenfile, err := os.ReadFile(goldenfilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}

		err = nil
	}

	var event github.Event
	if nil := json.Unmarshal(goldenfile, &event); err != nil {
		t.Fatal(err)
	}

	if err := handler.handleSource(
		context.TODO(),
		slog.Default(),
		team,
		team.Sources[0],
		event,
		github.TypeRepositoryRenamed,
	); err != nil {
		t.Error(err)
	}

	if len(slack.Messages) != 1 {
		t.Errorf("handleRenameEvent() did not send just one message: %d", len(slack.Messages))
	}
}
