package events

import (
	"context"
	"log/slog"
	"slices"
	"testing"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/mock"
	"github.com/navikt/ghep/internal/sql/gensql"
	"github.com/navikt/ghep/internal/testdata"
)

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

func TestHandleCommitEvents(t *testing.T) {
	db := &mock.Database{}
	slackClient := &mock.Slack{}
	team := github.Team{
		Name:          "test",
		SlackChannels: github.SlackChannels{},
		Config:        github.Config{},
		Sources: []github.Source{
			{
				SourceType: "commits",
				Channel:    "#test",
			},
		},
	}
	handler := NewHandler(db, slackClient, map[string]github.Team{"test": team})

	t.Run("Simple commit event", func(t *testing.T) {
		event, err := testdata.AsEvent("commit-1.json")
		if err != nil {
			t.Fatal(err)
		}

		if err := handler.handleSource(
			context.TODO(),
			slog.Default(),
			team,
			team.Sources[0],
			event,
		); err != nil {
			t.Error(err)
		}

		slackClient.Ensure(t, event.GetEventType(), 1, 0, 0)
	})
}
