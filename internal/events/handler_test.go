package events

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/mock"
)

func TestHandleCommitEvent(t *testing.T) {
	team := github.Team{
		Name: "test",
		SlackChannels: github.SlackChannels{
			Commits: "#test",
		},
	}

	type args struct {
		event github.Event
	}

	tests := []struct {
		name        string
		args        args
		wantPayload bool
	}{
		{
			name: "Kjent repo, feil branch",
			args: args{
				event: github.Event{
					Ref: "refs/heads/feature",
					Repository: &github.Repository{
						DefaultBranch: "main",
					},
				},
			},
			wantPayload: false,
		},
		{
			name: "Ingen commits",
			args: args{
				event: github.Event{
					Ref: "refs/heads/main",
					Repository: &github.Repository{
						DefaultBranch: "main",
					},
					Commits: []github.Commit{},
				},
			},
			wantPayload: false,
		},
		{
			name: "Kjent repo, riktig branch",
			args: args{
				event: github.Event{
					Ref: "refs/heads/main",
					Repository: &github.Repository{
						DefaultBranch: "main",
					},
					Commits: []github.Commit{
						{
							ID: "d6f21c84",
						},
					},
				},
			},
			wantPayload: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := handleCommitEvent(slog.Default(), team, tt.args.event, github.Client{})
			if err != nil {
				t.Error(err)
			}

			if tt.wantPayload && got == nil {
				t.Errorf("expected payload, got nil")
			}

			if !tt.wantPayload && got != nil {
				t.Errorf("expected no payload, got %v", got)
			}
		})
	}
}

func TestHandleIssueAndPullEvent(t *testing.T) {
	type args struct {
		team    github.Team
		mockSQL mock.TeamMock
		event   github.Event
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "No external channel, external sender",
			args: args{
				team: github.Team{
					Name: "test",
					SlackChannels: github.SlackChannels{
						Issues:       "#internal",
						PullRequests: "#internal",
					},
				},
				mockSQL: mock.TeamMock{
					Members: []string{"internal"},
				},
				event: github.Event{
					Action: "opened",
					Sender: github.User{
						Login: "external",
						Type:  "User",
					},
					Issue: &github.Issue{
						Number:      1,
						StateReason: "external",
					},
					PullRequest: &github.Issue{
						Number: 1,
					},
					Repository: &github.Repository{
						Name: "test",
					},
				},
			},
			want: "#internal",
		},
		{
			name: "External channel, external sender",
			args: args{
				team: github.Team{
					Name: "test",
					SlackChannels: github.SlackChannels{
						Issues:       "#internal",
						PullRequests: "#internal",
					},
					Config: github.Config{
						ExternalContributorsChannel: "#external",
					},
				},
				mockSQL: mock.TeamMock{
					Members: []string{"internal"},
				},
				event: github.Event{
					Action: "opened",
					Sender: github.User{
						Login: "external",
						Type:  "User",
					},
					Issue: &github.Issue{
						Number:      1,
						StateReason: "external",
					},
					PullRequest: &github.Issue{
						Number: 1,
					},
					Repository: &github.Repository{
						Name: "test",
					},
				},
			},
			want: "#external",
		},
		{
			name: "External channel, internal sender",
			args: args{
				team: github.Team{
					Name: "test",
					SlackChannels: github.SlackChannels{
						Issues:       "#internal",
						PullRequests: "#internal",
					},
					Config: github.Config{
						ExternalContributorsChannel: "#external",
					},
				},
				mockSQL: mock.TeamMock{
					Members: []string{"internal"},
				},
				event: github.Event{
					Action: "opened",
					Sender: github.User{
						Login: "internal",
						Type:  "User",
					},
					Issue: &github.Issue{
						Number:      1,
						StateReason: "external",
					},
					PullRequest: &github.Issue{
						Number: 1,
					},
					Repository: &github.Repository{
						Name: "test",
					},
				},
			},
			want: "#internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue, err := handleIssueEvent(context.Background(), slog.Default(), &tt.args.mockSQL, tt.args.team, "timestamp", tt.args.event)
			if err != nil {
				t.Error(err)
			}

			if diff := cmp.Diff(tt.want, issue.Channel); diff != "" {
				t.Errorf("handleIssueEvent() mismatch (-want +got):\n%s", diff)
			}

			pull, err := handlePullRequestEvent(context.Background(), slog.Default(), &tt.args.mockSQL, tt.args.team, "timestamp", tt.args.event)
			if err != nil {
				t.Error(err)
			}

			if diff := cmp.Diff(tt.want, pull.Channel); diff != "" {
				t.Errorf("handlePullRequestEvent() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHandleWorkflow(t *testing.T) {
	team := github.Team{
		Name: "test",
		SlackChannels: github.SlackChannels{
			Workflows: "#test",
		},
	}

	tests := []struct {
		name  string
		event github.Event
		team  github.Team
		err   bool
		want  []byte
	}{
		{
			name:  "No slack channel",
			event: github.Event{},
			team: github.Team{
				Name:          "test",
				SlackChannels: github.SlackChannels{},
			},
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
			team: team,
		},
		{
			name: "Not failure conclusion",
			event: github.Event{
				Action: "completed",
				Workflow: &github.Workflow{
					Conclusion: "success",
				},
			},
			team: team,
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
			team: team,
			want: []byte("test"),
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
			team: github.Team{
				Name: "test",
				SlackChannels: github.SlackChannels{
					Workflows: "#test",
				},
				Config: github.Config{
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
			team: github.Team{
				Name: "test",
				SlackChannels: github.SlackChannels{
					Workflows: "#test",
				},
				Config: github.Config{
					Workflows: github.Workflows{
						Branches: []string{"main"},
					},
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
			team: github.Team{
				Name: "test",
				SlackChannels: github.SlackChannels{
					Workflows: "#test",
				},
				Config: github.Config{
					Workflows: github.Workflows{
						Branches: []string{"main"},
					},
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
			team: github.Team{
				Name: "test",
				SlackChannels: github.SlackChannels{
					Workflows: "#test",
				},
				Config: github.Config{
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
			team: github.Team{
				Name: "test",
				SlackChannels: github.SlackChannels{
					Workflows: "#test",
				},
				Config: github.Config{
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
			team: github.Team{
				Name: "test",
				SlackChannels: github.SlackChannels{
					Workflows: "#test",
				},
				Config: github.Config{
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
			got, err := handleWorkflowEvent(slog.Default(), tt.team, tt.event)
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
