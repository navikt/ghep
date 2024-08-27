package github

// Generate a test for parseTeamConfig

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseTeamConfig(t *testing.T) {
	tests := []struct {
		name string
		path string
		want []Team
	}{
		{
			name: "team with all config",
			path: "testdata/all_config.yaml",
			want: []Team{
				{
					Name: "nada",
					SlackChannels: SlackChannels{
						Commits:      "#nada-test",
						Issues:       "#nada-test",
						PullRequests: "#nada-test",
						Workflows:    "#nada-test",
					},
					Config: Config{
						Workflows: Workflows{
							Branches:   []string{"main"},
							IgnoreBots: true,
						},
					},
				},
			},
		},
		{
			name: "team with minimal config",
			path: "testdata/minimal.yaml",
			want: []Team{
				{
					Name: "nada",
					SlackChannels: SlackChannels{
						Commits: "#nada-test",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseTeamConfig(test.path)
			if err != nil {
				t.Error(err)
			}

			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("parseTeamConfig mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
