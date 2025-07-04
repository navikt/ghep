package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/navikt/ghep/internal/sql"
	"gopkg.in/yaml.v3"
)

type Workflows struct {
	Branches     []string `yaml:"branches"`
	Repositories []string `yaml:"repositories"`
	Workflows    []string `yaml:"workflows"`
	IgnoreBots   bool     `yaml:"ignoreBots"`
}

type DependabotConfig string

const (
	DependabotConfigAlways DependabotConfig = "always"
)

type Config struct {
	ExternalContributorsChannel string           `yaml:"externalContributorsChannel"`
	Workflows                   Workflows        `yaml:"workflows"`
	SilenceDependabot           DependabotConfig `yaml:"silenceDependabot"`
	IgnoreRepositories          []string         `yaml:"ignoreRepositories"`
	Security                    Security         `yaml:"security"`
}

func (c Config) ShouldSilenceDependabot() bool {
	return c.SilenceDependabot == DependabotConfigAlways
}

type Security struct {
	SeverityFilter string `yaml:"severityFilter"`
}

func (s Security) SeverityType() SeverityType {
	return AsSeverityType(s.SeverityFilter)
}

type SlackChannels struct {
	Commits      string `yaml:"commits"`
	Issues       string `yaml:"issues"`
	PullRequests string `yaml:"pulls"`
	Releases     string `yaml:"releases"`
	Security     string `yaml:"security"`
	Workflows    string `yaml:"workflows"`
}

type Team struct {
	Name          string
	SlackChannels SlackChannels `yaml:",inline"`
	Config        Config        `yaml:"config"`
}

func fetchRepositories(teamURL, bearerToken string, blocklist []string) ([]string, error) {
	req, err := http.NewRequest("GET", teamURL+"/repos", nil)
	if err != nil {
		return nil, err
	}

	query := req.URL.Query()
	query.Set("per_page", "100")

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", bearerToken))
	req.Header.Add("Content-Type", "application/json")

	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}

	type GithubRepo struct {
		Name     string `json:"name"`
		Archived bool   `json:"archived"`
	}

	var repos []string
	page := 1
	for {
		query.Set("page", strconv.Itoa(page))
		req.URL.RawQuery = query.Encode()

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("error fetching repos (%v): %v", resp.Status, body)
		}

		var githubRepos []GithubRepo
		if err := json.Unmarshal(body, &githubRepos); err != nil {
			return nil, err
		}

		for _, repo := range githubRepos {
			if repo.Archived {
				continue
			}

			if slices.Contains(blocklist, repo.Name) {
				continue
			}

			repos = append(repos, repo.Name)
		}

		if len(githubRepos) < 100 {
			break
		}

		page++
	}

	return repos, nil
}

func parseTeamConfig(path string) (map[string]Team, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	teams := map[string]Team{}
	if err := yaml.NewDecoder(file).Decode(&teams); err != nil {
		return nil, fmt.Errorf("decoding team config: %v", err)
	}

	return teams, nil
}

func (c Client) ParseTeamConfig(ctx context.Context, teamsFilePath string) (map[string]Team, error) {
	teams, err := parseTeamConfig(teamsFilePath)
	if err != nil {
		return nil, fmt.Errorf("parsing team config: %v", err)
	}

	for name := range teams {
		if err := c.db.CreateTeam(ctx, name); err != nil {
			return nil, fmt.Errorf("creating team %s: %v", name, err)
		}
	}

	return teams, nil
}

func (c Client) FetchTeams(ctx context.Context, reposBlocklistString, orgTeam string, subscribeToOrg bool) error {
	bearerToken, err := c.createBearerToken()
	if err != nil {
		return fmt.Errorf("creating bearer token: %v", err)
	}

	reposBlocklist := strings.Split(reposBlocklistString, ",")

	if subscribeToOrg {
		team, err := c.db.GetTeam(ctx, orgTeam)
		if err != nil {
			return fmt.Errorf("getting team %s: %v", orgTeam, err)
		}

		url := fmt.Sprintf("https://api.github.com/orgs/%s", c.org)
		teamURL := fmt.Sprintf("%s/teams/%s", url, team)
		members, err := fetchMembers(teamURL, bearerToken)
		if err != nil {
			return fmt.Errorf("fetching members for %s: %v", team, err)
		}

		for _, member := range members {
			if err := sql.AddMemberToTeam(ctx, c.db, orgTeam, member.Login); err != nil {
				c.log.Error("Failed to add member to team", "team", orgTeam, "member", member.Login, "error", err)
				continue
			}
		}

		c.log.Info(fmt.Sprintf("Subscribed to %s", c.org), "org", c.org, "members", len(members))
		return nil
	}

	url := fmt.Sprintf("https://api.github.com/orgs/%s/teams", c.org)

	teams, err := c.db.ListTeams(ctx)
	if err != nil {
		return fmt.Errorf("listing teams from database: %v", err)
	}

	for _, team := range teams {
		teamURL := fmt.Sprintf("%s/%s", url, team)
		repositories, err := fetchRepositories(teamURL, bearerToken, reposBlocklist)
		if err != nil {
			c.log.Error("Failed fetching repositories", "team", team, "error", err)
			continue
		}

		for _, name := range repositories {
			if err := sql.AddRepositoryToTeam(ctx, c.db, team, name); err != nil {
				c.log.Error("Failed to add repository to team", "team", team, "repository", name, "error", err)
				continue
			}
		}

		members, err := fetchMembers(teamURL, bearerToken)
		if err != nil {
			return fmt.Errorf("fetching members for %s: %v", team, err)
		}

		for _, member := range members {
			if err := sql.AddMemberToTeam(ctx, c.db, team, member.Login); err != nil {
				c.log.Error("Failed to add member to team", "team", team, "member", member.Login, "error", err)
				continue
			}
		}
	}

	return nil
}
