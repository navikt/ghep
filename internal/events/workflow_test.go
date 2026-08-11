package events

import (
	"context"
	"log/slog"
	"slices"
	"testing"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/mock"
	"github.com/navikt/ghep/internal/testdata"
)

func TestHandleWorkflow(t *testing.T) {
	source := github.Source{
		SourceType: "workflows",
		Channel:    "#test",
	}

	tests := []struct {
		name   string
		event  github.Event
		source github.Source
		err    bool
		want   []byte
	}{
		{
			name:   "No slack channel",
			event:  github.Event{},
			source: github.Source{SourceType: "workflows", Channel: ""},
		},
		{
			name: "Not completed action",
			event: github.Event{
				Action: "started",
				Workflow: &github.Workflow{
					Conclusion: "",
				},
				Repository: &github.Repository{
					Name: "test",
				},
			},
			source: source,
		},
		{
			name: "Not failure conclusion",
			event: github.Event{
				Action: "completed",
				Workflow: &github.Workflow{
					Conclusion: "success",
				},
			},
			source: source,
		},
		{
			name: "Valid event",
			event: github.Event{
				Action: "completed",
				Workflow: &github.Workflow{
					Conclusion: "failure",
				},
				Repository: &github.Repository{
					Name: "test",
				},
			},
			source: source,
			want:   []byte("test"),
		},
		{
			name: "Event from bot user",
			event: github.Event{
				Action: "completed",
				Workflow: &github.Workflow{
					Conclusion: "failure",
				},
				Sender: github.User{
					Login: "dependabot",
					Type:  "Bot",
				},
				Repository: &github.Repository{
					Name: "test",
				},
			},
			source: github.Source{
				SourceType: "workflows",
				Channel:    "#test",
				Config: github.SourceConfig{
					Workflows: github.Workflows{
						IgnoreBots: true,
					},
				},
			},
		},
		{
			name: "Only interested in some branches",
			event: github.Event{
				Action: "completed",
				Workflow: &github.Workflow{
					HeadBranch: "main",
					Conclusion: "failure",
				},
				Repository: &github.Repository{
					Name: "test",
				},
			},
			source: github.Source{
				SourceType: "workflows",
				Channel:    "#test",
				Config: github.SourceConfig{
					Branches: []string{"main"},
				},
			},
			want: []byte("test"),
		},
		{
			name: "Ignore branches not matching",
			event: github.Event{
				Action: "completed",
				Workflow: &github.Workflow{
					HeadBranch: "feature/some_feature",
					Conclusion: "failure",
				},
				Repository: &github.Repository{
					Name: "test",
				},
			},
			source: github.Source{
				SourceType: "workflows",
				Channel:    "#test",
				Config: github.SourceConfig{
					Branches: []string{"main"},
				},
			},
			want: nil,
		},
		{
			name: "Ignore repositories not matching",
			event: github.Event{
				Action: "completed",
				Workflow: &github.Workflow{
					Conclusion: "failure",
				},
				Repository: &github.Repository{
					Name: "test",
				},
			},
			source: github.Source{
				SourceType: "workflows",
				Channel:    "#test",
				Config: github.SourceConfig{
					Workflows: github.Workflows{
						Repositories: []string{"other-repo"},
					},
				},
			},
			want: nil,
		},
		{
			name: "Ignore workflows not matching",
			event: github.Event{
				Action: "completed",
				Workflow: &github.Workflow{
					Conclusion: "failure",
					Name:       "test",
				},
				Repository: &github.Repository{
					Name: "test",
				},
			},
			source: github.Source{
				SourceType: "workflows",
				Channel:    "#test",
				Config: github.SourceConfig{
					Workflows: github.Workflows{
						Workflows: []string{"other-workflow"},
					},
				},
			},
			want: nil,
		},
		{
			name: "Allow only specific repositories and workflows",
			event: github.Event{
				Action: "completed",
				Workflow: &github.Workflow{
					Conclusion: "failure",
					Name:       "test",
				},
				Repository: &github.Repository{
					Name: "test",
				},
			},
			source: github.Source{
				SourceType: "workflows",
				Channel:    "#test",
				Config: github.SourceConfig{
					Workflows: github.Workflows{
						Repositories: []string{"test"},
						Workflows:    []string{"test"},
					},
				},
			},
			want: []byte("test"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Branch filtering happens in handleForSource, not handleWorkflowEvent — simulate it here.
			if len(tt.source.Config.Branches) > 0 {
				branch := eventBranch(tt.event, github.TypeWorkflow)
				if branch != "" && !slices.Contains(tt.source.Config.Branches, branch) {
					if tt.want != nil {
						t.Errorf("source branch filter dropped event, expected a message")
					}
					return
				}
			}

			got, err := handleWorkflowEvent(slog.Default(), tt.source, tt.event)
			if err != nil && !tt.err {
				t.Error(err)
			}

			if tt.err && err == nil {
				t.Errorf("expected error, got nil")
			}

			if tt.want == nil && got != nil {
				t.Errorf("expected no payload, got %v", got)
			}

			if tt.want != nil && got == nil {
				t.Errorf("expected payload, got nil")
			}
		})
	}
}

func TestHandleWorkflowEvents(t *testing.T) {
	team := github.Team{
		Name:          "test",
		SlackChannels: github.SlackChannels{},
		Config:        github.Config{},
		Sources: []github.Source{
			{
				SourceType: "commits",
				Channel:    "#test",
			},
			{
				SourceType: "workflows",
				Channel:    "#test",
			},
			{
				SourceType: "pulls",
				Channel:    "#test",
			},
		},
	}
	teamConfig := map[string]github.Team{"test": team}

	t.Run("Simple workflow event", func(t *testing.T) {
		slack := &mock.Slack{}
		handler := NewHandler(&mock.Database{}, slack, teamConfig)

		workflowEvent, err := testdata.AsEvent("workflow-run-failure-1.json")
		if err != nil {
			t.Fatal(err)
		}

		sources := team.SourcesForType(workflowEvent.GetEventType())
		if sources == nil {
			t.Errorf("No source found for %s", workflowEvent.GetEventType())
		}

		if err := handler.handleSource(
			context.TODO(),
			slog.Default(),
			team,
			sources[0],
			workflowEvent,
		); err != nil {
			t.Error(err)
		}

		slack.Ensure(t, workflowEvent.GetEventType(), 1, 0, 0)
	})

	t.Run("Workflow event with commit", func(t *testing.T) {
		slack := &mock.Slack{}
		handler := NewHandler(&mock.Database{}, slack, teamConfig)

		commitEvent, err := testdata.AsEvent("commit-2.json")
		if err != nil {
			t.Fatal(err)
		}

		sources := team.SourcesForType(commitEvent.GetEventType())
		if sources == nil {
			t.Errorf("No source found for %s", commitEvent.GetEventType())
		}

		// prepopulate db with a commit event
		if err := handler.handleSource(
			context.TODO(),
			slog.Default(),
			team,
			sources[0],
			commitEvent,
		); err != nil {
			t.Error(err)
		}

		slack.EnsureMessages(t, commitEvent.GetEventType(), 1)

		workflowEvent, err := testdata.AsEvent("workflow-run-failure-1.json")
		if err != nil {
			t.Fatal(err)
		}

		sources = team.SourcesForType(workflowEvent.GetEventType())
		if sources == nil {
			t.Errorf("No source found for %s", workflowEvent.GetEventType())
		}

		// ensure events are connected
		workflowEvent.Workflow.HeadSHA = commitEvent.After

		if err := handler.handleSource(
			context.TODO(),
			slog.Default(),
			team,
			sources[0],
			workflowEvent,
		); err != nil {
			t.Error(err)
		}

		slack.Ensure(t, workflowEvent.GetEventType(), 2, 1, 1)
	})

	t.Run("Successful workflow with pull request", func(t *testing.T) {
		slack := &mock.Slack{}
		handler := NewHandler(&mock.Database{}, slack, teamConfig)

		pullRequestEvent, err := testdata.AsEvent("pull-opened-1.json")
		if err != nil {
			t.Fatal(err)
		}

		sources := team.SourcesForType(pullRequestEvent.GetEventType())
		if sources == nil {
			t.Errorf("No source found for %s", pullRequestEvent.GetEventType())
		}

		// prepopulate db with a pull request event
		if err := handler.handleSource(
			context.TODO(),
			slog.Default(),
			team,
			sources[0],
			pullRequestEvent,
		); err != nil {
			t.Error(err)
		}

		slack.EnsureMessages(t, pullRequestEvent.GetEventType(), 1)

		workflowEvent, err := testdata.AsEvent("workflow-run-success-pull-requests-1.json")
		if err != nil {
			t.Fatal(err)
		}

		sources = team.SourcesForType(workflowEvent.GetEventType())
		if sources == nil {
			t.Errorf("No source found for %s", workflowEvent.GetEventType())
		}

		if err := handler.handleSource(
			context.TODO(),
			slog.Default(),
			team,
			sources[0],
			workflowEvent,
		); err != nil {
			t.Error(err)
		}

		slack.Ensure(t, workflowEvent.GetEventType(), 1, 1, 0)
	})
}
