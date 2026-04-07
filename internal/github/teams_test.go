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
					Sources: []Source{
						{SourceType: "commits", Channel: "#commits"},
						{SourceType: "pulls", Channel: "#pulls"},
						{SourceType: "issues", Channel: "#issues"},
						{
							SourceType: "workflows",
							Channel:    "#workflows",
							Config: SourceConfig{
								Workflows: Workflows{
									IgnoreBots:   true,
									Repositories: []string{"my-little-repo"},
									Workflows:    []string{"deploy"},
								},
							},
						},
						{SourceType: "releases", Channel: "#releases"},
						{
							SourceType: "security",
							Channel:    "#security",
							Config: SourceConfig{
								Security: Security{
									SeverityFilter: "high",
								},
							},
						},
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
					Sources: []Source{
						{SourceType: "commits", Channel: "#nada-test"},
					},
				},
			},
		},
		{
			name: "team with sources config",
			path: "testdata/sources_config.yaml",
			want: map[string]Team{
				"nada": {
					Name: "nada",
					Config: Config{
						ExternalContributorsChannel: "#external",
						SilenceDependabot:           "always",
						IgnoreRepositories:          []string{"repoA"},
						PingSlackUsers:              true,
					},
					Sources: []Source{
						{
							SourceType: "pulls",
							Channel:    "#pr-main",
							Config: SourceConfig{
								Pulls: PullsConfig{
									IgnoreBots:   true,
									IgnoreDrafts: true,
									Minimalist:   true,
								},
							},
						},
						{SourceType: "pulls", Channel: "#pr-all"},
						{
							SourceType: "pulls",
							Channel:    "#pr-open",
							Config: SourceConfig{
								Pulls: PullsConfig{
									IgnoreDrafts: true,
									Events: []string{"opened", "ready_for_review"},
								},
							},
						},
						{
							SourceType: "pulls",
							Channel:    "#pr-merged",
							Config: SourceConfig{
								Pulls: PullsConfig{
									Events: []string{"merged"},
								},
							},
						},
						{SourceType: "commits", Channel: "#commits"},
						{
							SourceType: "commits",
							Channel:    "#commits-develop",
							Config: SourceConfig{
								Branches: []string{"develop", "staging"},
							},
						},
						{
							SourceType: "workflows",
							Channel:    "#ci",
							Config: SourceConfig{
								Branches: []string{"main"},
								Workflows: Workflows{
									IgnoreBots: true,
								},
							},
						},
						{SourceType: "releases", Channel: "#releases"},
						{SourceType: "issues", Channel: "#issues"},
						{
							SourceType: "security",
							Channel:    "#security",
							Config: SourceConfig{
								Security: Security{
									SeverityFilter: "high",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "hybrid config: flat channels + explicit sources",
			path: "testdata/hybrid_config.yaml",
			want: map[string]Team{
				"nada": {
					Name: "nada",
					SlackChannels: SlackChannels{
						Commits:      "#commits",
						PullRequests: "#pr-default",
					},
					Sources: []Source{
						{SourceType: "commits", Channel: "#commits"},
						{SourceType: "pulls", Channel: "#pr-default"},
						{
							SourceType: "pulls",
							Channel:    "#pr-bots",
							Config: SourceConfig{
								Pulls: PullsConfig{
									IgnoreBots: false,
								},
							},
						},
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
