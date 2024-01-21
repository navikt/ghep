package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDataDirectory = "../../testdata"

func TestCreateCommitEvent(t *testing.T) {
	files, err := os.ReadDir(testDataDirectory)
	if err != nil {
		t.Error(err)
		t.Fail()
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if !strings.HasPrefix(name, "commit-1.json") {
			continue
		}

		path := filepath.Join(testDataDirectory, name)

		t.Run(path, func(t *testing.T) {
			testdata, err := os.ReadFile(path)
			if err != nil {
				t.Error(err)
			}

			event, err := CreateCommitEvent(testdata)
			if err != nil {
				t.Error(err)
			}

			if event.Repository.Name != "crm-nks-integration" {
				t.Errorf("expected repository name to be 'crm-nks-integration', got '%v'", event.Repository.Name)
			}

			if len(event.Commits) != 11 {
				t.Errorf("expected 1 commit, got %v", len(event.Commits))
			}

			commit := event.Commits[0]
			if commit.Id != "b7" {
				t.Errorf("expected commit id to be 'b7', got '%v'", commit.Id)
			}

			if commit.Author.Name != "Ola Nordmann" {
				t.Errorf("expected commit author to be 'Ola Nordmann', got '%v'", commit.Author.Name)
			}
		})
	}
}
