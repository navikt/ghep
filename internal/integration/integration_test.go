package integration

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
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
	oldMessage := slack.Message{
		Channel: slackChannel,
		Text:    "Should be updated",
		Attachments: []slack.Attachment{
			{
				Text:  "Should be updated",
				Color: "#34a44c",
			},
		},
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

			goldenfilePath := filepath.Join("testdata/output", entry.Name())
			goldenfile, err := os.ReadFile(goldenfilePath)
			if err != nil {
				if !os.IsNotExist(err) {
					t.Fatal(err)
				}

				err = nil
			}

			var message *slack.Message
			eventType := event.GetEventType()
			switch eventType {
			case github.TypeCommit:
				team := github.Team{
					Members: []*github.User{
						{
							Login: "Kyrremann",
							URL:   "https://github.com/Kyrremann",
						},
					},
				}

				message, err = slack.CreateCommitMessage(slog.Default(), slackChannel, event, team, mockhub)
			case github.TypeIssue:
				if event.Action == "edited" {
					message = slack.CreateUpdatedIssueMessage(oldMessage, event)
				} else {
					message = slack.CreateIssueMessage(slackChannel, "", event)
				}
			case github.TypePullRequest:
				if event.Action == "edited" {
					message = slack.CreateUpdatedPullRequestMessage(oldMessage, event)
				} else {
					message = slack.CreatePullRequestMessage(slackChannel, "", event)
				}
			case github.TypeRepositoryRenamed:
				message = slack.CreateRenamedMessage(slackChannel, event)
			case github.TypeRepositoryPublic:
				message = slack.CreatePublicizedMessage(slackChannel, event)
			case github.TypeTeam:
				message = slack.CreateTeamMessage(slackChannel, event)
			case github.TypeWorkflow:
				event.Workflow.FailedJob = github.FailedJob{
					Name: "job",
					URL:  "https://url.com",
					Step: "step",
				}

				message = slack.CreateWorkflowMessage(slackChannel, event)
			case github.TypeRelease:
				var timestamp string
				if event.Action == "edited" {
					timestamp = "1234567890"
				}

				message = slack.CreateReleaseMessage(slackChannel, timestamp, event)
			default:
				t.Fatalf("unknown event file: %s", entry.Name())
			}

			if err != nil {
				t.Fatalf("err should be nil, should be checked closer to action: %s", err)
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
				if got.String() != "" {
					// Probably a new test, output the new golden file
					t.Logf("Got: %s", got)
				}
				// t.Logf("Golden file: %s", goldenfile)
			}
		})
	}
}
