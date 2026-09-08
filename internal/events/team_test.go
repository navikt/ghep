package events

import (
	"context"
	"log/slog"
	"testing"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/mock"
)

func TestHandleTeamSideEffectsAddedToRepository(t *testing.T) {
	tests := []struct {
		name        string
		ignoreForks bool
		fork        bool
		wantAdded   bool
	}{
		{
			name:        "regular repo is added",
			ignoreForks: false,
			fork:        false,
			wantAdded:   true,
		},
		{
			name:        "fork is added when team does not ignore forks",
			ignoreForks: false,
			fork:        true,
			wantAdded:   true,
		},
		{
			name:        "regular repo is added when team ignores forks",
			ignoreForks: true,
			fork:        false,
			wantAdded:   true,
		},
		{
			name:        "fork is skipped when team ignores forks",
			ignoreForks: true,
			fork:        true,
			wantAdded:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := &mock.Database{}
			teamsConfig := map[string]github.Team{
				"nada": {
					Name: "nada",
					Config: github.Config{
						IgnoreForks: tt.ignoreForks,
					},
				},
			}

			handler := NewHandler(mockDB, nil, teamsConfig)
			event := github.Event{
				Action: "added_to_repository",
				Team: &github.TeamEvent{
					Name: "nada",
				},
				Repository: &github.Repository{
					Name: "my-repo",
					Fork: tt.fork,
				},
			}

			if err := handler.handleTeamSideEffects(context.Background(), slog.Default(), event); err != nil {
				t.Fatal(err)
			}

			gotAdded := len(mockDB.TeamRepositories) > 0
			if gotAdded != tt.wantAdded {
				t.Errorf("repository added = %v, want %v", gotAdded, tt.wantAdded)
			}
		})
	}
}
