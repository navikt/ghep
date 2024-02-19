package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

func TestHandleEvent(t *testing.T) {
	dir, err := os.ReadDir("testdata/events")
	if err != nil {
		t.Error(err)
	}

	for _, entry := range dir {
		if entry.IsDir() {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			testdataPath := filepath.Join("testdata/events", entry.Name())
			testdata, err := os.ReadFile(testdataPath)
			if err != nil {
				t.Fatal(err)
			}

			event, err := github.CreateEvent(testdata)
			if err != nil {
				t.Fatal(err)
			}

			goldnefilePath := filepath.Join("testdata/output", entry.Name())
			goldenfile, err := os.ReadFile(goldnefilePath)
			if err != nil {
				t.Fatal(err)
			}

			var got []byte
			switch strings.Split(entry.Name(), "-")[0] {
			case "commit":
				tmpl, err := template.ParseFiles("../slack/templates/commit.tmpl")
				if err != nil {
					t.Fatal(err)
				}

				got, err = slack.CreateCommitMessage(*tmpl, "#test", event)
				if err != nil {
					t.Fatal(err)
				}
			case "issue":
				tmpl, err := template.ParseFiles("../slack/templates/issue.tmpl")
				if err != nil {
					t.Fatal(err)
				}

				got, err = slack.CreateIssueMessage(*tmpl, "#test", "", event)
				if err != nil {
					t.Fatal(err)
				}
			case "pull":
				tmpl, err := template.ParseFiles("../slack/templates/pull.tmpl")
				if err != nil {
					t.Fatal(err)
				}

				got, err = slack.CreatePullRequestMessage(*tmpl, "#test", "", event)
				if err != nil {
					t.Fatal(err)
				}
			case "team":
				tmpl, err := template.ParseFiles("../slack/templates/team.tmpl")
				if err != nil {
					t.Fatal(err)
				}

				got, err = slack.CreateTeamMessage(*tmpl, "#test", event)
				if err != nil {
					t.Fatal(err)
				}
			default:
				t.Skipf("unknown event file: %s", entry.Name())
			}

			if ok := json.Valid(got); !ok {
				t.Fatalf("invalid json: %s", got)
			}

			if diff := cmp.Diff(string(goldenfile), string(got)); diff != "" {
				t.Errorf("CreateCommitEvent() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
