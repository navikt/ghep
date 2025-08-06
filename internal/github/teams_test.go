package github

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseTeamConfig(t *testing.T) {
	tests := []struct {
		name string
		path string
		want map[string]Team
	}{
		{
			name: "team with all config",
			path: "testdata/all_config.yaml",
			want: map[string]Team{
				"nada": {
					Name: "nada",
					SlackChannels: SlackChannels{
						Commits:      "#commits",
						Issues:       "#issues",
						PullRequests: "#pulls",
						Workflows:    "#workflows",
						Releases:     "#releases",
						Security:     "#security",
					},
					Config: Config{
						Workflows: Workflows{
							Branches:     []string{"main"},
							IgnoreBots:   true,
							Repositories: []string{"my-little-repo"},
							Workflows:    []string{"deploy"},
						},
						ExternalContributorsChannel: "#external",
						SilenceDependabot:           "always",
						IgnoreRepositories:          []string{"repoA"},
						Security: Security{
							SeverityFilter: "high",
						},
						PingSlackUsers: true,
					},
				},
			},
		},
		{
			name: "team with minimal config",
			path: "testdata/minimal.yaml",
			want: map[string]Team{
				"nada": {
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
			got, err := ParseTeamConfig(test.path)
			if err != nil {
				t.Error(err)
			}

			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("parseTeamConfig mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
