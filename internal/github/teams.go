package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Workflows struct {
	Branches   []string `yaml:"branches"`
	IgnoreBots bool     `yaml:"ignoreBots"`
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
}

func (c Config) SilenceDepedabot() bool {
	return c.SilenceDependabot == DependabotConfigAlways
}

type SlackChannels struct {
	Commits      string `yaml:"commits"`
	Issues       string `yaml:"issues"`
	PullRequests string `yaml:"pulls"`
	Workflows    string `yaml:"workflows"`
}

type Team struct {
	Name          string
	Repositories  []string
	Members       []User
	SlackChannels SlackChannels `yaml:",inline"`
	Config        Config        `yaml:"config"`
}

func (t *Team) GetMemberByName(name string) (User, bool) {
	for _, member := range t.Members {
		if member.Login == name {
			return member, true
		}
	}

	return User{}, false
}

func (t *Team) IsMember(user string) bool {
	contains := func(u User) bool {
		return u.Login == user
	}

	return slices.ContainsFunc(t.Members, contains)
}

func (t *Team) AddRepository(repo string) {
	t.Repositories = append(t.Repositories, repo)
}

func (t *Team) RemoveRepository(remove string) {
	for i, repo := range t.Repositories {
		if repo == remove {
			t.Repositories = append(t.Repositories[:i], t.Repositories[i+1:]...)
			return
		}
	}
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
			return nil, fmt.Errorf("error fetching repos (%v): %v", resp.Status, resp.Body)
		}

		githubRepos := []GithubRepo{}
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

// fetchMembers fetches members for a Github org or team using the following API:
// Team: https://docs.github.com/en/rest/teams/members#list-team-members
// Org: https://docs.github.com/en/rest/orgs/members#list-organization-members
func fetchMembers(teamURL, bearerToken string) ([]User, error) {
	req, err := http.NewRequest("GET", teamURL+"/members", nil)
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

	var teamMembers []User
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
			return nil, fmt.Errorf("%s returned %v: %v", teamURL, resp.Status, body)
		}

		var members []User
		if err := json.Unmarshal(body, &members); err != nil {
			return nil, err
		}

		teamMembers = append(teamMembers, members...)

		if len(members) < 100 {
			break
		}

		page++
	}

	return teamMembers, nil
}

func parseTeamConfig(path string) ([]Team, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	teamsAsMap := map[string]Team{}
	if err := yaml.NewDecoder(file).Decode(&teamsAsMap); err != nil {
		return nil, fmt.Errorf("decoding team config: %v", err)
	}

	teams := make([]Team, 0, len(teamsAsMap))
	for name, team := range teamsAsMap {
		team.Name = name
		teams = append(teams, team)
	}

	return teams, nil
}

func (c Client) FetchTeams(teamsFilePath, reposBlocklistString, subscribeToOrg string) ([]Team, error) {
	teams, err := parseTeamConfig(teamsFilePath)
	if err != nil {
		return nil, fmt.Errorf("parsing team config: %v", err)
	}

	bearerToken, err := c.createBearerToken()
	if err != nil {
		return nil, fmt.Errorf("creating bearer token: %v", err)
	}

	reposBlocklist := strings.Split(reposBlocklistString, ",")

	if subscribeToOrg == "true" {
		url := fmt.Sprintf("https://api.github.com/orgs/%s", c.org)
		repos, err := fetchRepositories(url, bearerToken, reposBlocklist)
		if err != nil {
			return nil, err
		}
		teams[0].Repositories = repos

		teamUrl := fmt.Sprintf("%s/teams/%s", url, teams[0].Name)
		members, err := fetchMembers(teamUrl, bearerToken)
		if err != nil {
			return nil, fmt.Errorf("fetching members for %s: %v", teams[0].Name, err)
		}
		teams[0].Members = members

		c.log.Info(fmt.Sprintf("Subscribed to %s", c.org), "org", c.org, "repositories", len(repos), "members", len(members))
		return teams, nil
	}

	url := fmt.Sprintf("https://api.github.com/orgs/%s/teams", c.org)

	for i, team := range teams {
		teamUrl := fmt.Sprintf("%s/%s", url, team.Name)
		repos, err := fetchRepositories(teamUrl, bearerToken, reposBlocklist)
		if err != nil {
			return nil, err
		}
		team.Repositories = repos

		members, err := fetchMembers(teamUrl, bearerToken)
		if err != nil {
			return nil, fmt.Errorf("fetching members for %s: %v", team.Name, err)
		}
		team.Members = members

		teams[i] = team
	}

	return teams, nil
}
