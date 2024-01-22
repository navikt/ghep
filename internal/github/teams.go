package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

type SlackChannels struct {
	Commits      string `yaml:"commits"`
	Issues       string `yaml:"issues"`
	PullRequests string `yaml:"pulls"`
}

type Team struct {
	Name          string
	Repositories  []string
	SlackChannels SlackChannels
}

const githubAPITeamEndpointTmpl = "{{ .url }}/orgs/{{ .org }}/teams/{{ .team }}/repos?per_page=100"

func fetchTeamsRepositories(url, bearerToken string) ([]string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", bearerToken))
	req.Header.Add("Content-Type", "application/json")

	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}

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

	type GithubRepo struct {
		Name     string `json:"name"`
		Archived bool   `json:"archived"`
	}

	githubRepos := []GithubRepo{}
	if err := json.Unmarshal(body, &githubRepos); err != nil {
		return nil, err
	}

	var repos []string
	for _, repo := range githubRepos {
		if repo.Archived {
			continue
		}

		repos = append(repos, repo.Name)
	}

	return repos, nil
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

func FetchTeams(githubAPI, appInstallationID, appID, appPrivateKeyFilePath, githubOrg, teamsFilePath string) ([]Team, error) {
	tmpl, err := template.New("github").Parse(githubAPITeamEndpointTmpl)
	if err != nil {
		return nil, err
	}

	tmplData := map[string]string{
		"url": githubAPI,
		"org": githubOrg,
	}

	bearerToken, err := createBearerToken(githubAPI, appInstallationID, appID, appPrivateKeyFilePath)
	if err != nil {
		return nil, err
	}

	teamsChannels, err := getTeamsChannels(teamsFilePath)
	if err != nil {
		return nil, err
	}

	var teams []Team
	for name, team := range teamsChannels {
		tmplData["team"] = name

		var url strings.Builder
		if err := tmpl.Execute(&url, tmplData); err != nil {
			return nil, err
		}

		repos, err := fetchTeamsRepositories(url.String(), bearerToken)
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
