package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

const (
	testdataEventsPath = "testdata/events"
	slackChannel       = "#test"
)

func TestHandleEvent(t *testing.T) {
	slackTemplates, err := slack.ParseMessageTemplates()
	if err != nil {
		t.Fatal(err)
	}

	dir, err := os.ReadDir(testdataEventsPath)
	if err != nil {
		t.Error(err)
	}

	for _, entry := range dir {
		if entry.IsDir() {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			testdataPath := filepath.Join(testdataEventsPath, entry.Name())
			testdata, err := os.ReadFile(testdataPath)
			if err != nil {
				t.Fatal(err)
			}

			event, err := github.CreateEvent(testdata)
			if err != nil {
				t.Fatal(err)
			}

			goldnefilePath := filepath.Join("testdata/output", entry.Name())
			goldenfile, err := os.ReadFile(goldnefilePath)
			if err != nil {
				t.Fatal(err)
			}

			var got []byte
			switch strings.Split(entry.Name(), "-")[0] {
			case "commit":
				team := github.Team{
					Members: []github.User{
						{
							Login: "Kyrremann",
							URL:   "https://github.com/Kyrremann",
						},
					},
				}

				got, err = slack.CreateCommitMessage(slackTemplates["commit"], slackChannel, event, team)
				if err != nil {
					t.Fatal(err)
				}
			case "issue":
				got, err = slack.CreateIssueMessage(slackTemplates["issue"], slackChannel, "", event)
				if err != nil {
					t.Fatal(err)
				}
			case "pull":
				got, err = slack.CreatePullRequestMessage(slackTemplates["pull"], slackChannel, "", event)
				if err != nil {
					t.Fatal(err)
				}
			case "team":
				got, err = slack.CreateTeamMessage(slackTemplates["team"], slackChannel, event)
				if err != nil {
					t.Fatal(err)
				}
			case "workflow":
				got, err = slack.CreateWorkflowMessage(slackTemplates["workflow"], slackChannel, event)
				if err != nil {
					t.Fatal(err)
				}
			default:
				t.Skipf("unknown event file: %s", entry.Name())
			}

			if ok := json.Valid(got); !ok {
				t.Fatalf("invalid json: %s", got)
			}

			if diff := cmp.Diff(string(goldenfile), string(got)); diff != "" {
				t.Errorf("Create Slack message mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
