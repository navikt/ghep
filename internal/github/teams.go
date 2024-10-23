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
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

type client struct {
	apiURL            string
	appInstallationID string
	appID             string
	appPrivateKey     string
	org               string
}

const githubAPITeamEndpointTmpl = "{{ .url }}/orgs/{{ .org }}/teams/{{ .team }}"

type Workflows struct {
	Branches   []string `yaml:"branches"`
	IgnoreBots bool     `yaml:"ignoreBots"`
}

type Config struct {
	ExternalContributorsChannel string    `yaml:"externalContributorsChannel"`
	Workflows                   Workflows `yaml:"workflows"`
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

func (t Team) GetMemberByName(name string) (User, bool) {
	for _, member := range t.Members {
		if member.Login == name {
			return member, true
		}
	}

	return User{}, false
}

func (t Team) IsMember(user string) bool {
	contains := func(u User) bool {
		return u.Login == user
	}

	return slices.ContainsFunc(t.Members, contains)
}

func fetchTeamsRepositories(teamURL, bearerToken string, blocklist []string) ([]string, error) {
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

// fetchTeamMembers fetches members for a Github team using the following API:
// https://docs.github.com/en/rest/teams/members#list-team-members
func fetchTeamMembers(teamURL, bearerToken string) ([]User, error) {
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
			return nil, fmt.Errorf("error fetching repos (%v): %v", resp.Status, resp.Body)
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

func (c client) FetchTeams(teamsFilePath, reposBlocklistString string) ([]Team, error) {
	teams, err := parseTeamConfig(teamsFilePath)
	if err != nil {
		return nil, fmt.Errorf("parsing team config: %v", err)
	}

	tmpl, err := template.New("github").Parse(githubAPITeamEndpointTmpl)
	if err != nil {
		return nil, err
	}

	tmplData := map[string]string{
		"url": c.apiURL,
		"org": c.org,
	}

	bearerToken, err := c.createBearerToken()
	if err != nil {
		return nil, fmt.Errorf("creating bearer token: %v", err)
	}

	reposBlocklist := strings.Split(reposBlocklistString, ",")

	for i, team := range teams {
		tmplData["team"] = team.Name

		var url strings.Builder
		if err := tmpl.Execute(&url, tmplData); err != nil {
			return nil, err
		}

		repos, err := fetchTeamsRepositories(url.String(), bearerToken, reposBlocklist)
		if err != nil {
			return nil, err
		}

		team.Repositories = repos

		members, err := fetchTeamMembers(url.String(), bearerToken)
		if err != nil {
			return nil, err
		}

		team.Members = members

		teams[i] = team
	}

	return teams, nil
}

func New(githubAPI, appInstallationID, appID, appPrivateKey, githubOrg string) client {
	return client{
		apiURL:            githubAPI,
		appInstallationID: appInstallationID,
		appID:             appID,
		appPrivateKey:     appPrivateKey,
		org:               githubOrg,
	}
}
