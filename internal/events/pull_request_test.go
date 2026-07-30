package events

import (
	"context"
	"log/slog"
	"testing"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/mock"
)

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
		{name: "onlyMerged: closed merged", events: []string{"merged"}, event: newPR("merged", true), wantMessage: true},
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
