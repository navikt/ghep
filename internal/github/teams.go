package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

type SlackChannels struct {
	Commits      string `yaml:"commits"`
	Issues       string `yaml:"issues"`
	PullRequests string `yaml:"pulls"`
	Workflows    string `yaml:"workflows"`
}

type Team struct {
	Name          string
	Repositories  []string
	SlackChannels SlackChannels
}

const githubAPITeamEndpointTmpl = "{{ .url }}/orgs/{{ .org }}/teams/{{ .team }}/repos"

func fetchTeamsRepositories(teamURL, bearerToken string, blocklist []string) ([]string, error) {
	req, err := http.NewRequest("GET", teamURL, nil)
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

			if contains(blocklist, repo.Name) {
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

func contains(s []string, e string) bool {
	for _, a := range s {
		if strings.EqualFold(a, e) {
			return true
		}
	}
	return false
}

func getTeamsChannels(teamsFilePath string) (map[string]SlackChannels, error) {
	teamsFile, err := os.Open(teamsFilePath)
	if err != nil {
		return nil, err
	}

	channels := map[string]SlackChannels{}
	if err := yaml.NewDecoder(teamsFile).Decode(&channels); err != nil {
		return nil, err
	}

	return channels, nil
}

func FetchTeams(githubAPI, appInstallationID, appID, appPrivateKey, githubOrg, teamsFilePath, reposBlocklistString string) ([]Team, error) {
	tmpl, err := template.New("github").Parse(githubAPITeamEndpointTmpl)
	if err != nil {
		return nil, err
	}

	tmplData := map[string]string{
		"url": githubAPI,
		"org": githubOrg,
	}

	bearerToken, err := createBearerToken(githubAPI, appInstallationID, appID, appPrivateKey)
	if err != nil {
		return nil, err
	}

	teamsChannels, err := getTeamsChannels(teamsFilePath)
	if err != nil {
		return nil, fmt.Errorf("creating bearer token: %v", err)
	}

	reposBlocklist := strings.Split(reposBlocklistString, ",")

	var teams []Team
	for name, team := range teamsChannels {
		tmplData["team"] = name

		var url strings.Builder
		if err := tmpl.Execute(&url, tmplData); err != nil {
			return nil, err
		}

		repos, err := fetchTeamsRepositories(url.String(), bearerToken, reposBlocklist)
		if err != nil {
			return nil, err
		}

		teams = append(teams, Team{
			Name:          name,
			Repositories:  repos,
			SlackChannels: team,
		})
	}

	return teams, nil
}
