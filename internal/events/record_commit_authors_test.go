package events

import (
	"log/slog"
	"testing"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/mock"
)

func TestRecordCommitAuthors(t *testing.T) {
	tests := []struct {
		name  string
		users []string
		event github.Event
		want  map[string]int32 // login -> total upserted commit count
	}{
		{
			name:  "known author is recorded",
			users: []string{"kyrremann"},
			event: pushEvent(
				github.Commit{Author: github.Author{Username: "kyrremann"}},
			),
			want: map[string]int32{"kyrremann": 1},
		},
		{
			name:  "author casing differs from stored login",
			users: []string{"Kyrremann"},
			event: pushEvent(
				github.Commit{Author: github.Author{Username: "kyrremann"}},
			),
			want: map[string]int32{"kyrremann": 1},
		},
		{
			name:  "unknown author is skipped",
			users: []string{"kyrremann"},
			event: pushEvent(
				github.Commit{Author: github.Author{Username: "external-contributor"}},
			),
			want: map[string]int32{},
		},
		{
			name:  "known co-author is recorded, unknown co-author is skipped",
			users: []string{"kyrremann", "thokra-nav"},
			event: pushEvent(
				github.Commit{
					Author:  github.Author{Username: "kyrremann"},
					Message: "did stuff\n\nCo-authored-by: @thokra-nav <thokra-nav@users.noreply.github.com>\nCo-authored-by: @outsider <outsider@users.noreply.github.com>",
				},
			),
			want: map[string]int32{"kyrremann": 1, "thokra-nav": 1},
		},
		{
			name:  "bots are not checked against users",
			users: []string{"kyrremann"},
			event: pushEvent(
				github.Commit{Author: github.Author{Username: "dependabot[bot]"}},
				github.Commit{Author: github.Author{Username: "kyrremann"}},
			),
			want: map[string]int32{"kyrremann": 1},
		},
		{
			name:  "multiple commits by same author are summed",
			users: []string{"kyrremann"},
			event: pushEvent(
				github.Commit{Author: github.Author{Username: "kyrremann"}},
				github.Commit{Author: github.Author{Username: "kyrremann"}},
			),
			want: map[string]int32{"kyrremann": 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &mock.Database{Users: tt.users}

			recordCommitAuthors(slog.Default(), db, tt.event)

			if len(db.CommitCountUpserts) != len(tt.want) {
				t.Fatalf("got %d upserts, want %d: %+v", len(db.CommitCountUpserts), len(tt.want), db.CommitCountUpserts)
			}

			for _, upsert := range db.CommitCountUpserts {
				want, ok := tt.want[upsert.Login]
				if !ok {
					t.Errorf("unexpected upsert for login %q", upsert.Login)
					continue
				}

				if upsert.CommitCount != want {
					t.Errorf("login %q: got commit count %d, want %d", upsert.Login, upsert.CommitCount, want)
				}

				if upsert.Repo != tt.event.Repository.Name {
					t.Errorf("login %q: got repo %q, want %q", upsert.Login, upsert.Repo, tt.event.Repository.Name)
				}
			}
		})
	}
}

func pushEvent(commits ...github.Commit) github.Event {
	return github.Event{
		Repository: &github.Repository{Name: "ghep"},
		Commits:    commits,
	}
}
