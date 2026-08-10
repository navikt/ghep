package events

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/mock"
	"github.com/navikt/ghep/internal/slack"
)

func TestEventBranch(t *testing.T) {
	tests := []struct {
		name      string
		event     github.Event
		eventType github.EventType
		want      string
	}{
		{
			name:      "commit on main",
			eventType: github.TypeCommit,
			event:     github.Event{Ref: "refs/heads/main"},
			want:      "main",
		},
		{
			name:      "commit on feature branch",
			eventType: github.TypeCommit,
			event:     github.Event{Ref: "refs/heads/feature/my-feature"},
			want:      "feature/my-feature",
		},
		{
			name:      "workflow on main",
			eventType: github.TypeWorkflow,
			event:     github.Event{Workflow: &github.Workflow{HeadBranch: "main"}},
			want:      "main",
		},
		{
			name:      "workflow with no workflow data",
			eventType: github.TypeWorkflow,
			event:     github.Event{},
			want:      "",
		},
		{
			name:      "pull request targeting main",
			eventType: github.TypePullRequest,
			event:     github.Event{PullRequest: &github.Issue{Base: github.IssueBase{Ref: "main"}}},
			want:      "main",
		},
		{
			name:      "pull request with no PR data",
			eventType: github.TypePullRequest,
			event:     github.Event{},
			want:      "",
		},
		{
			name:      "issue has no branch",
			eventType: github.TypeIssue,
			event:     github.Event{},
			want:      "",
		},
		{
			name:      "release has no branch",
			eventType: github.TypeRelease,
			event:     github.Event{},
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eventBranch(tt.event, tt.eventType)
			if got != tt.want {
				t.Errorf("eventBranch() = %q, want %q", got, tt.want)
			}
		})
	}
}

const (
	testdataEventsPath  = "../testdata/events"
	testdataOutputsPath = "../testdata/output"
	slackChannel        = "#test"
)

func TestHandleEvent(t *testing.T) {
	ctx := context.Background()
	log := slog.Default()
	mockDB := &mock.Database{}
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

			goldenfilePath := filepath.Join(testdataOutputsPath, entry.Name())
			goldenfile, err := os.ReadFile(goldenfilePath)
			if err != nil {
				if !os.IsNotExist(err) {
					t.Fatal(err)
				}

				err = nil
			}

			pingSlack := true
			var message *slack.Message
			switch event.GetEventType() {
			case github.TypeCommit:
				message, err = slack.CreateCommitMessage(ctx, log, mockDB, slackChannel, event)
			case github.TypeIssue:
				message = slack.CreateIssueMessage(ctx, log, mockDB, slackChannel, "", pingSlack, event)
			case github.TypePullRequest:
				minimalist := false
				if event.PullRequest.Merged {
					event.Action = "merged"
				}
				message = slack.CreatePullRequestMessage(ctx, log, mockDB, slackChannel, "", pingSlack, minimalist, event)
			case github.TypePullRequestReview:
				return // no-op for Slack
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
				message = slack.CreateCodeScanningAlertMessage(slackChannel, "", event)
			case github.TypeDependabotAlert:
				message = slack.CreateDependabotAlertMessage(slackChannel, "", event)
			case github.TypeSecurityAdvisory:
				message = slack.CreateSecurityAdvisoryMessage(slackChannel, event)
			case github.TypeSecretScanningAlert:
				message = slack.CreateSecretScanningAlertMessage(slackChannel, "", event)
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
			}
		})
	}
}
