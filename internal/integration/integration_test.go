package integration

import (
	"bytes"
	"encoding/json"
	"log/slog"
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
	mockhub := mockHub{}

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

			var message *slack.Message
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

				message, err = slack.CreateCommitMessage(slog.Default(), slackChannel, event, team, mockhub)
				if err != nil {
					t.Fatal(err)
				}
			case "issue":
				message = slack.CreateIssueMessage(slackChannel, "", event)
			case "pull":
				message = slack.CreatePullRequestMessage(slackChannel, "", event)
			case "removed":
				message = slack.CreateRemovedMessage(slackChannel, event)
			case "renamed":
				message = slack.CreateRenamedMessage(slackChannel, event)
			case "public":
				message = slack.CreatePublicizedMessage(slackChannel, event)
			case "team":
				message = slack.CreateTeamMessage(slackChannel, event)
			case "workflow":
				event.Workflow.FailedJob = github.FailedJob{
					Name: "job",
					URL:  "https://url.com",
					Step: "step",
				}

				message = slack.CreateWorkflowMessage(slackChannel, event)
				if err != nil {
					t.Fatal(err)
				}
			default:
				t.Skipf("unknown event file: %s", entry.Name())
			}

			got := new(bytes.Buffer)
			enc := json.NewEncoder(got)
			enc.SetEscapeHTML(false)
			enc.SetIndent("", "  ")
			if err := enc.Encode(message); err != nil {
				t.Fatal(err)
			}

			if ok := json.Valid(got.Bytes()); !ok {
				t.Fatalf("invalid json: %s", got)
			}

			if diff := cmp.Diff(string(goldenfile), got.String()); diff != "" {
				t.Errorf("Create Slack message mismatch (-want +got):\n%s", diff)
				t.Logf("Golden file: %s", goldenfile)
				t.Logf("Got: %s", got)
			}
		})
	}
}
