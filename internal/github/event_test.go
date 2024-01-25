package github

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCreateCommitEvent(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     Event
	}{
		{
			name:     "Test create commit event",
			filename: "commit-1.json",
			want: Event{
				Commits: []Commit{
					{
						ID: "2df91bf91d56ec91e64fb8c60e779ab548b4d599",
					},
					{
						ID: "c08bcc4ee8c8b951319244c470f182496d4e0c23",
					},
				},
				Repository: Repository{
					Name:          "knorten",
					DefaultBranch: "main",
				},
				Sender: Sender{
					Login: "Kyrremann",
				},
				Ref: "refs/heads/main",
			},
		},
	}

	opt := cmp.FilterPath(func(p cmp.Path) bool {
		vx := p.String()

		if vx == "Commits.URL" ||
			vx == "Commits.Message" ||
			vx == "Sender.URL" ||
			vx == "Repository.URL" ||
			vx == "Compare" {
			return true
		}
		return false
	}, cmp.Ignore())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join("testdata", tt.filename)

			testdata, err := os.ReadFile(path)
			if err != nil {
				t.Error(err)
			}

			got, err := CreateEvent(testdata)
			if err != nil {
				t.Error(err)
			}

			if diff := cmp.Diff(tt.want, got, opt); diff != "" {
				t.Errorf("CreateCommitEvent() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
