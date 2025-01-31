package events

import (
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/ghep/internal/github"
)

func TestHandleCommitEvent(t *testing.T) {
	team := github.Team{
		Name: "test",
		SlackChannels: github.SlackChannels{
			Commits: "#test",
		},
		Repositories: []string{"test"},
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
					Repository: github.Repository{
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
					Repository: github.Repository{
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
					Repository: github.Repository{
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
		team  github.Team
		event github.Event
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
					Members: []github.User{{Login: "internal"}},
				},
				event: github.Event{
					Action: "opened",
					Sender: github.User{
						Login: "external",
					},
					Issue: &github.Issue{
						Number:      1,
						StateReason: "external",
					},
					PullRequest: &github.Issue{
						Number: 1,
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
					Members: []github.User{{Login: "internal"}},
					Config: github.Config{
						ExternalContributorsChannel: "#external",
					},
				},
				event: github.Event{
					Action: "opened",
					Sender: github.User{
						Login: "external",
					},
					Issue: &github.Issue{
						Number:      1,
						StateReason: "external",
					},
					PullRequest: &github.Issue{
						Number: 1,
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
					Members: []github.User{{Login: "internal"}},
					Config: github.Config{
						ExternalContributorsChannel: "#external",
					},
				},
				event: github.Event{
					Action: "opened",
					Sender: github.User{
						Login: "internal",
					},
					Issue: &github.Issue{
						Number:      1,
						StateReason: "external",
					},
					PullRequest: &github.Issue{
						Number: 1,
					},
				},
			},
			want: "#internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue, err := handleIssueEvent(slog.Default(), tt.args.team, "", tt.args.event)
			if err != nil {
				t.Error(err)
			}

			if diff := cmp.Diff(tt.want, issue.Channel); diff != "" {
				t.Errorf("handleIssueEvent() mismatch (-want +got):\n%s", diff)
			}

			pull, err := handlePullRequestEvent(slog.Default(), tt.args.team, "", tt.args.event)
			if err != nil {
				t.Error(err)
			}

			if diff := cmp.Diff(tt.want, pull.Channel); diff != "" {
				t.Errorf("handlePullRequestEvent() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHandleTeamEvent(t *testing.T) {
	type args struct {
		team  github.Team
		event github.Event
	}

	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Added to repository",
			args: args{
				team: github.Team{
					Name: "test",
					SlackChannels: github.SlackChannels{
						Commits: "#test",
					},
					Repositories: []string{"test"},
				},
				event: github.Event{
					Action: "added_to_repository",
					Team: &github.TeamEvent{
						Name: "test",
					},
					Repository: github.Repository{
						Name:     "new-repo",
						RoleName: "admin",
					},
				},
			},
			want: []string{"test", "new-repo"},
		},
		{
			name: "Removed from repository",
			args: args{
				team: github.Team{
					Name: "test",
					SlackChannels: github.SlackChannels{
						Commits: "#test",
					},
					Repositories: []string{"test", "new-repo"},
				},
				event: github.Event{
					Action: "removed_from_repository",
					Team: &github.TeamEvent{
						Name: "test",
					},
					Repository: github.Repository{
						Name:     "new-repo",
						RoleName: "admin",
					},
				},
			},
			want: []string{"test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handleTeamEvent(slog.Default(), &tt.args.team, tt.args.event)
			if err != nil {
				t.Error(err)
			}

			if diff := cmp.Diff(tt.want, tt.args.team.Repositories); diff != "" {
				t.Errorf("repositories mismatch (-want +got):\n%s", diff)
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
					Type: "Bot",
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
					Conclusion: "success",
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
