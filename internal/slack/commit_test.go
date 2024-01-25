package slack

import (
	"os"
	"testing"
	"text/template"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/ghep/internal/github"
)

func TestCreateCommitMessage(t *testing.T) {
	commitTmpl, err := template.ParseFiles("templates/commit.tmpl")
	if err != nil {
		t.Error(err)
	}

	want, err := os.ReadFile("goldenfiles/commit.json")
	if err != nil {
		t.Error(err)
	}

	event := github.Event{
		Compare: "https://github.com/test/compare/2d7f6c9...d6f21c8",
		Repository: github.Repository{
			Name: "test",
			URL:  "https://github.com/test",
		},
		Commits: []github.Commit{
			{
				ID:      "d6f21c84",
				Message: "test",
				URL:     "https://github.com/test",
			},
		},
		Sender: github.Sender{
			Login: "Ola Nordmann",
			URL:   "https://github.com/olanordmann",
		},
	}

	t.Run("test", func(t *testing.T) {
		got, err := CreateCommitMessage(commitTmpl, "#test", event)
		if err != nil {
			t.Error(err)
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("payload mismatch (-want +got):\n%s", diff)
		}
	})
}
