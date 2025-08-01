package integration

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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
				Color: slack.ColorOpened,
			},
		},
	}
	oldMessageWithReviewers := slack.Message{
		Channel: slackChannel,
		Text:    "Should be updated",
		Attachments: []slack.Attachment{
			{
				Text:  "There should be no reviewers\n*Requested reviewers:* @Kyrremann",
				Color: slack.ColorOpened,
			},
		},
	}
	oldMessageWithAssignees := slack.Message{
		Channel: slackChannel,
		Text:    "Should be updated",
		Attachments: []slack.Attachment{
			{
				Text:  "There should be no assignees\n*Assignees:* @Kyrremann",
				Color: slack.ColorOpened,
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
				message, err = slack.CreateCommitMessage(slog.Default(), slackChannel, event, mockhub)
			case github.TypeIssue:
				if slices.Contains([]string{"opened", "closed", "reopened"}, event.Action) {
					message = slack.CreateIssueMessage(slackChannel, "", event)
				} else {
					if event.Action == "unassigned" {
						message = slack.CreateUpdatedIssueMessage(oldMessageWithAssignees, event)
					} else {
						message = slack.CreateUpdatedIssueMessage(oldMessage, event)
					}
				}
			case github.TypePullRequest:
				if slices.Contains([]string{"opened", "closed", "reopened"}, event.Action) {
					message = slack.CreatePullRequestMessage(slackChannel, "", event)
				} else {
					if event.Action == "review_request_removed" {
						message = slack.CreateUpdatedPullRequestMessage(oldMessageWithReviewers, event)
					} else {
						message = slack.CreateUpdatedPullRequestMessage(oldMessage, event)
					}
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
				message = slack.CreateReleaseMessage(slackChannel, event)
			case github.TypeCodeScanningAlert:
				message = slack.CreateCodeScanningAlertMessage(slackChannel, event)
			case github.TypeDependabotAlert:
				message = slack.CreateDependabotAlertMessage(slackChannel, event, "")
			case github.TypeSecurityAdvisory:
				message = slack.CreateSecurityAdvisoryMessage(slackChannel, event)
			case github.TypeSecretScanningAlert:
				message = slack.CreateSecretScanningAlertMessage(slackChannel, event)
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
