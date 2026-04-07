package events

import (
	"context"
	"log/slog"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/mock"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func TestHandleCommitEvent(t *testing.T) {
	source := github.Source{
		SourceType: "commits",
		Channel:    "#test",
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
			got, err := handleCommitEvent(context.Background(), slog.Default(), source, tt.args.event, &gensql.Queries{})
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
		team   github.Team
		source github.Source
		mockDB mock.Database
		event  github.Event
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
				},
				source: github.Source{
					SourceType: "issues",
					Channel:    "#internal",
				},
				mockDB: mock.Database{
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
					Config: github.Config{
						ExternalContributorsChannel: "#external",
					},
				},
				source: github.Source{
					SourceType: "issues",
					Channel:    "#internal",
				},
				mockDB: mock.Database{
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
					Config: github.Config{
						ExternalContributorsChannel: "#external",
					},
				},
				source: github.Source{
					SourceType: "issues",
					Channel:    "#internal",
				},
				mockDB: mock.Database{
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
			issueSource := tt.args.source
			issueSource.SourceType = "issues"
			issue, err := handleIssueEvent(context.Background(), slog.Default(), &tt.args.mockDB, tt.args.team, issueSource, "timestamp", tt.args.event)
			if err != nil {
				t.Error(err)
			}

			if diff := cmp.Diff(tt.want, issue.Channel); diff != "" {
				t.Errorf("handleIssueEvent() mismatch (-want +got):\n%s", diff)
			}

			pullSource := tt.args.source
			pullSource.SourceType = "pulls"
			pull, err := handlePullRequestEvent(context.Background(), slog.Default(), &tt.args.mockDB, tt.args.team, pullSource, "timestamp", tt.args.event)
			if err != nil {
				t.Error(err)
			}

			if diff := cmp.Diff(tt.want, pull.Channel); diff != "" {
				t.Errorf("handlePullRequestEvent() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHandlePullRequestBotFilter(t *testing.T) {
	team := github.Team{Name: "test"}

	tests := []struct {
		name        string
		source      github.Source
		event       github.Event
		wantMessage bool
	}{
		{
			name: "onlyBots: human sender, bot PR author (Dependabot merged by human) - should send",
			source: github.Source{
				SourceType: "pulls",
				Channel:    "#bots",
				Config: github.SourceConfig{
					Pulls: github.PullsConfig{OnlyBots: true},
				},
			},
			event: github.Event{
				Action: "closed",
				Sender: github.User{Login: "human-user", Type: "User"},
				PullRequest: &github.Issue{
					Number: 1,
					User:   github.User{Login: "dependabot[bot]", Type: "Bot"},
					Merged: true,
				},
				Repository: &github.Repository{Name: "test"},
			},
			wantMessage: true,
		},
		{
			name: "onlyBots: human sender, human PR author - should not send",
			source: github.Source{
				SourceType: "pulls",
				Channel:    "#bots",
				Config: github.SourceConfig{
					Pulls: github.PullsConfig{OnlyBots: true},
				},
			},
			event: github.Event{
				Action: "opened",
				Sender: github.User{Login: "human-user", Type: "User"},
				PullRequest: &github.Issue{
					Number: 2,
					User:   github.User{Login: "human-user", Type: "User"},
				},
				Repository: &github.Repository{Name: "test"},
			},
			wantMessage: false,
		},
		{
			name: "ignoreBots: human sender, bot PR author (Dependabot merged by human) - should not send",
			source: github.Source{
				SourceType: "pulls",
				Channel:    "#humans",
				Config: github.SourceConfig{
					Pulls: github.PullsConfig{IgnoreBots: true},
				},
			},
			event: github.Event{
				Action: "closed",
				Sender: github.User{Login: "human-user", Type: "User"},
				PullRequest: &github.Issue{
					Number: 3,
					User:   github.User{Login: "dependabot[bot]", Type: "Bot"},
					Merged: true,
				},
				Repository: &github.Repository{Name: "test"},
			},
			wantMessage: false,
		},
		{
			name: "ignoreBots: bot sender, human PR author - should not send",
			source: github.Source{
				SourceType: "pulls",
				Channel:    "#humans",
				Config: github.SourceConfig{
					Pulls: github.PullsConfig{IgnoreBots: true},
				},
			},
			event: github.Event{
				Action: "opened",
				Sender: github.User{Login: "dependabot[bot]", Type: "Bot"},
				PullRequest: &github.Issue{
					Number: 4,
					User:   github.User{Login: "human-user", Type: "User"},
				},
				Repository: &github.Repository{Name: "test"},
			},
			wantMessage: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &mock.Database{Members: []string{}}
			msg, err := handlePullRequestEvent(context.Background(), slog.Default(), db, team, tt.source, "", tt.event)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.wantMessage && msg == nil {
				t.Errorf("expected a message, got nil")
			}
			if !tt.wantMessage && msg != nil {
				t.Errorf("expected no message, got %+v", msg)
			}
		})
	}
}

func TestHandlePullRequestEventsFilter(t *testing.T) {
	team := github.Team{Name: "test"}
	db := &mock.Database{Members: []string{}}

	newPR := func(action string, merged bool) github.Event {
		return github.Event{
			Action: action,
			Sender: github.User{Login: "human", Type: "User"},
			PullRequest: &github.Issue{
				Number: 1,
				User:   github.User{Login: "human", Type: "User"},
				Merged: merged,
			},
			Repository: &github.Repository{Name: "test"},
		}
	}

	tests := []struct {
		name        string
		events      []string
		event       github.Event
		wantMessage bool
	}{
		// No filter (default) — all three actions produce a message
		{name: "no filter: opened", events: nil, event: newPR("opened", false), wantMessage: true},
		{name: "no filter: ready_for_review", events: nil, event: newPR("ready_for_review", false), wantMessage: true},
		{name: "no filter: closed merged", events: nil, event: newPR("closed", true), wantMessage: true},
		{name: "no filter: closed not merged", events: nil, event: newPR("closed", false), wantMessage: true},

		// events: [opened, ready_for_review] — req. 1
		{name: "onlyOpen: opened", events: []string{"opened", "ready_for_review"}, event: newPR("opened", false), wantMessage: true},
		{name: "onlyOpen: ready_for_review", events: []string{"opened", "ready_for_review"}, event: newPR("ready_for_review", false), wantMessage: true},
		{name: "onlyOpen: closed merged", events: []string{"opened", "ready_for_review"}, event: newPR("closed", true), wantMessage: false},
		{name: "onlyOpen: closed not merged", events: []string{"opened", "ready_for_review"}, event: newPR("closed", false), wantMessage: false},

		// events: [merged] — req. 2
		{name: "onlyMerged: opened", events: []string{"merged"}, event: newPR("opened", false), wantMessage: false},
		{name: "onlyMerged: ready_for_review", events: []string{"merged"}, event: newPR("ready_for_review", false), wantMessage: false},
		{name: "onlyMerged: closed merged", events: []string{"merged"}, event: newPR("closed", true), wantMessage: true},
		{name: "onlyMerged: closed not merged", events: []string{"merged"}, event: newPR("closed", false), wantMessage: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := github.Source{
				SourceType: "pulls",
				Channel:    "#test",
				Config: github.SourceConfig{
					Pulls: github.PullsConfig{Events: tt.events},
				},
			}
			msg, err := handlePullRequestEvent(context.Background(), slog.Default(), db, team, source, "", tt.event)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantMessage && msg == nil {
				t.Errorf("expected a message, got nil")
			}
			if !tt.wantMessage && msg != nil {
				t.Errorf("expected no message, got %+v", msg)
			}
		})
	}
}

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

func TestHandleCommitEventBranchFilter(t *testing.T) {
	tests := []struct {
		name        string
		source      github.Source
		event       github.Event
		wantMessage bool
	}{
		{
			name: "no config.branches: push to default branch - should send",
			source: github.Source{
				SourceType: "commits",
				Channel:    "#commits",
			},
			event: github.Event{
				Ref:        "refs/heads/main",
				Repository: &github.Repository{DefaultBranch: "main"},
				Commits:    []github.Commit{{ID: "d6f21c84"}},
			},
			wantMessage: true,
		},
		{
			name: "no config.branches: push to non-default branch - should not send",
			source: github.Source{
				SourceType: "commits",
				Channel:    "#commits",
			},
			event: github.Event{
				Ref:        "refs/heads/develop",
				Repository: &github.Repository{DefaultBranch: "main"},
				Commits:    []github.Commit{{ID: "d6f21c84"}},
			},
			wantMessage: false,
		},
		{
			name: "config.branches set: push to listed branch - should send",
			source: github.Source{
				SourceType: "commits",
				Channel:    "#commits",
				Config:     github.SourceConfig{Branches: []string{"develop", "staging"}},
			},
			event: github.Event{
				Ref:        "refs/heads/develop",
				Repository: &github.Repository{DefaultBranch: "main"},
				Commits:    []github.Commit{{ID: "d6f21c84"}},
			},
			wantMessage: true,
		},
		{
			name: "config.branches set: push to unlisted branch - should not send",
			source: github.Source{
				SourceType: "commits",
				Channel:    "#commits",
				Config:     github.SourceConfig{Branches: []string{"develop", "staging"}},
			},
			event: github.Event{
				Ref:        "refs/heads/feature/xyz",
				Repository: &github.Repository{DefaultBranch: "main"},
				Commits:    []github.Commit{{ID: "d6f21c84"}},
			},
			wantMessage: false,
		},
		{
			name: "config.branches set with non-default branch: overrides default-branch check",
			source: github.Source{
				SourceType: "commits",
				Channel:    "#release-commits",
				Config:     github.SourceConfig{Branches: []string{"release"}},
			},
			event: github.Event{
				Ref:        "refs/heads/release",
				Repository: &github.Repository{DefaultBranch: "main"},
				Commits:    []github.Commit{{ID: "d6f21c84"}},
			},
			wantMessage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the handleForSource branch filter before calling the handler
			if len(tt.source.Config.Branches) > 0 {
				branch := eventBranch(tt.event, github.TypeCommit)
				if branch != "" && !slices.Contains(tt.source.Config.Branches, branch) {
					if tt.wantMessage {
						t.Errorf("source branch filter dropped event, expected a message")
					}
					return
				}
			}

			msg, err := handleCommitEvent(context.Background(), slog.Default(), tt.source, tt.event, &gensql.Queries{})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.wantMessage && msg == nil {
				t.Errorf("expected a message, got nil")
			}
			if !tt.wantMessage && msg != nil {
				t.Errorf("expected no message, got %+v", msg)
			}
		})
	}
}

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
