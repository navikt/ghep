package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

// IgnoreThreshold checks if the severity is below the configured threshold.
// Example: if the severity filter is "high", it will ignore "medium" and "low" severities.
func (s Security) IgnoreThreshold(incomingSeverity string) bool {
	return AsSeverityType(incomingSeverity) < s.SeverityType()
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

type githubError struct {
	Message string `json:"message"`
	Status  string `json:"status"`
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
			var ghErr githubError
			if err := json.Unmarshal(body, &ghErr); err != nil {
				return nil, fmt.Errorf("error fetching repos (%v): %s", resp.Status, body)
			}

			return nil, fmt.Errorf("error fetching repos (%v): %s (%s)", resp.Status, ghErr.Message, ghErr.Status)
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

func ParseTeamConfig(path string) (map[string]Team, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	teams := map[string]Team{}
	if err := yaml.NewDecoder(file).Decode(&teams); err != nil {
		return nil, fmt.Errorf("decoding team config: %v", err)
	}

	for name, team := range teams {
		team.Name = name
		teams[name] = team
	}

	return teams, nil
}

func validateOrgExists(url, bearerToken string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %v", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", bearerToken))
	req.Header.Add("Content-Type", "application/json")

	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}

	return nil
}

func validateTeamExists(teamURL, bearerToken string) error {
	req, err := http.NewRequest("GET", teamURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %v", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", bearerToken))
	req.Header.Add("Content-Type", "application/json")

	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}

	return nil
}

func (c Client) FetchOrgMembersAsTeam(ctx context.Context, log *slog.Logger) error {
	bearerToken, err := c.createBearerToken()
	if err != nil {
		return fmt.Errorf("creating bearer token: %v", err)
	}

	team, err := c.db.GetTeam(ctx, c.org)
	if err != nil {
		return fmt.Errorf("getting team %s: %v", c.org, err)
	}

	url := fmt.Sprintf("https://api.github.com/orgs/%s", c.org)
	if err := validateOrgExists(url, bearerToken); err != nil {
		return fmt.Errorf("validating organization %s: %v", c.org, err)
	}

	teamURL := fmt.Sprintf("%s/teams/%s", url, team)
	members, err := fetchMembers(teamURL, bearerToken)
	if err != nil {
		return fmt.Errorf("fetching members for %s: %v", team, err)
	}

	for _, member := range members {
		if err := sql.AddMemberToTeam(ctx, c.db, c.org, member.Login); err != nil {
			log.Error("Failed to add member to team", "team", c.org, "member", member.Login, "error", err)
			continue
		}
	}

	log.Info(fmt.Sprintf("Subscribed to %s", c.org), "org", c.org, "members", len(members))

	return nil
}

func (c Client) FetchTeams(ctx context.Context, log *slog.Logger, reposBlocklistString string) error {
	bearerToken, err := c.createBearerToken()
	if err != nil {
		return fmt.Errorf("creating bearer token: %v", err)
	}

	reposBlocklist := strings.Split(reposBlocklistString, ",")
	url := fmt.Sprintf("https://api.github.com/orgs/%s/teams", c.org)

	teams, err := c.db.ListTeams(ctx)
	if err != nil {
		return fmt.Errorf("listing teams from database: %v", err)
	}

	for _, team := range teams {
		teamURL := fmt.Sprintf("%s/%s", url, team)
		if err := validateTeamExists(teamURL, bearerToken); err != nil {
			log.Error("Team does not exist", "team", team, "error", err)
			continue
		}

		repositories, err := fetchRepositories(teamURL, bearerToken, reposBlocklist)
		if err != nil {
			return fmt.Errorf("fetching repositories for %s: %v", team, err)
		}

		for _, repository := range repositories {
			if err := sql.AddRepositoryToTeam(ctx, c.db, team, repository); err != nil {
				return fmt.Errorf("adding repository %s to team %s: %v", repository, team, err)
			}
		}

		members, err := fetchMembers(teamURL, bearerToken)
		if err != nil {
			return fmt.Errorf("fetching members for %s: %v", team, err)
		}

		for _, member := range members {
			if err := sql.AddMemberToTeam(ctx, c.db, team, member.Login); err != nil {
				return fmt.Errorf("adding member %s to team %s: %v", member.Login, team, err)
			}
		}

		log.Info(fmt.Sprintf("Processed team %s with %d repositories and %d members", team, len(repositories), len(members)))
	}

	return nil
}
