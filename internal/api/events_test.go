package api

import (
	"testing"
	"text/template"

	"github.com/navikt/ghep/internal/github"
)

func TestHandleCommitEvent(t *testing.T) {
	tmpl, err := template.New("dummy").Parse("test")
	if err != nil {
		t.Fatal(err)
	}

	team := github.Team{
		Name: "test",
		SlackChannels: github.SlackChannels{
			Commits: "#test",
		},
		Repositories: []string{"test"},
	}

	type args struct {
		event github.Event
	}

	tests := []struct {
		name        string
		args        args
		wantPayload bool
	}{
		{
			name: "Kjent repo, feil branch",
			args: args{
				event: github.Event{
					Ref: "refs/heads/feature",
					Repository: github.Repository{
						DefaultBranch: "main",
					},
				},
			},
			wantPayload: false,
		},
		{
			name: "Ingen commits",
			args: args{
				event: github.Event{
					Ref: "refs/heads/main",
					Repository: github.Repository{
						DefaultBranch: "main",
					},
					Commits: []github.Commit{},
				},
			},
			wantPayload: false,
		},
		{
			name: "Kjent repo, riktig branch",
			args: args{
				event: github.Event{
					Ref: "refs/heads/main",
					Repository: github.Repository{
						DefaultBranch: "main",
					},
					Commits: []github.Commit{
						{
							ID: "d6f21c84",
						},
					},
				},
			},
			wantPayload: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := handleCommitEvent(*tmpl, team, tt.args.event)
			if err != nil {
				t.Error(err)
			}

			if tt.wantPayload && got == nil {
				t.Errorf("expected payload, got nil")
			}

			if !tt.wantPayload && got != nil {
				t.Errorf("expected no payload, got %v", got)
			}
		})
	}
}
