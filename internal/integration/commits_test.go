package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
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
			tmpl, err := template.ParseFiles("../slack/templates/commit.tmpl")
			if err != nil {
				t.Fatal(err)
			}

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

			got, err := slack.CreateCommitMessage(*tmpl, "#test", event)
			if err != nil {
				t.Fatal(err)
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
