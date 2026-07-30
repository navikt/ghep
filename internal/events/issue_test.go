package events

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/mock"
)

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
