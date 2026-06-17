package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/navikt/ghep/internal/sql"
	"gopkg.in/yaml.v3"
)

type Workflows struct {
	Repositories []string `yaml:"repositories"`
	Workflows    []string `yaml:"workflows"`
	IgnoreBots   bool     `yaml:"ignoreBots"`
}

type DependabotConfig string

const (
	DependabotConfigAlways DependabotConfig = "always"

	TeamNameExternalContributors = "external-contributors"
)

type Config struct {
	ExternalContributorsChannel string           `yaml:"externalContributorsChannel"`
	Workflows                   Workflows        `yaml:"workflows"`
	SilenceDependabot           DependabotConfig `yaml:"silenceDependabot"`
	IgnoreRepositories          []string         `yaml:"ignoreRepositories"`
	Security                    Security         `yaml:"security"`
	PingSlackUsers              bool             `yaml:"pingSlackUsers"`
	Pulls                       PullsConfig      `yaml:"pulls"`
}

type PullsConfig struct {
	IgnoreBots   bool     `yaml:"ignoreBots"`
	OnlyBots     bool     `yaml:"onlyBots"`
	IgnoreDrafts bool     `yaml:"ignoreDrafts"`
	Minimalist   bool     `yaml:"minimalist"`
	Events       []string `yaml:"events"`
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

// SourceConfig holds event-type-specific config for a source.
type SourceConfig struct {
	Branches  []string    `yaml:"branches"`
	Pulls     PullsConfig `yaml:"pulls"`
	Workflows Workflows   `yaml:"workflows"`
	Security  Security    `yaml:"security"`
}

// Source defines a single event-type-to-channel mapping with optional config.
type Source struct {
	SourceType string       `yaml:"source"`
	Channel    string       `yaml:"channel"`
	Config     SourceConfig `yaml:"config"`
}

func (t Team) IsExternalContributor() bool {
	return t.Name == "external-contributors"
}

type DigestConfig struct {
	Channel            string   `yaml:"channel"`
	Day                string   `yaml:"day"`
	Time               string   `yaml:"time"`
	Timezone           string   `yaml:"timezone"`
	SendEmpty          bool     `yaml:"send_empty"`
	SpecifyTeamName    bool     `yaml:"specifyTeamName"`
	IgnoreRepositories []string `yaml:"ignoreRepositories"`
}

type SecurityDigestConfig struct {
	Channel            string   `yaml:"channel"`
	Day                string   `yaml:"day"`
	Time               string   `yaml:"time"`
	Timezone           string   `yaml:"timezone"`
	SendEmpty          bool     `yaml:"send_empty"`
	SpecifyTeamName    bool     `yaml:"specifyTeamName"`
	SeverityFilter     string   `yaml:"severity_filter"`
	IgnoreRepositories []string `yaml:"ignoreRepositories"`
}

func TitleCaseSlug(slug string) string {
	words := strings.Split(slug, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}

	return strings.Join(words, " ")
}

func (s SecurityDigestConfig) SeverityType() SeverityType {
	return AsSeverityType(s.SeverityFilter)
}

type Team struct {
	Name              string
	SlackChannels     SlackChannels         `yaml:",inline"`
	Config            Config                `yaml:"config"`
	Sources           []Source              `yaml:"sources"`
	PullRequestDigest *DigestConfig         `yaml:"pr-digest"`
	SecurityDigest    *SecurityDigestConfig `yaml:"security-digest"`
}

// SourcesForType returns all sources matching the given event type.
func (t Team) SourcesForType(eventType EventType) []Source {
	var sourceType string
	switch eventType {
	case TypeCommit:
		sourceType = "commits"
	case TypeIssue:
		sourceType = "issues"
	case TypePullRequest:
		sourceType = "pulls"
	case TypeWorkflow:
		sourceType = "workflows"
	case TypeRelease:
		sourceType = "releases"
	case TypeCodeScanningAlert, TypeDependabotAlert, TypeSecretScanningAlert, TypeSecurityAdvisory:
		sourceType = "security"
	case TypeRepositoryRenamed, TypeRepositoryPublic:
		sourceType = "commits"
	default:
		return nil
	}

	var sources []Source
	for _, s := range t.Sources {
		if s.SourceType == sourceType {
			sources = append(sources, s)
		}
	}
	return sources
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
		resp.Body.Close() // #nosec G104 -- closing response body, error intentionally ignored
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

type teamsFile struct {
	PersonalDigest []PersonalDigestUserEntry `yaml:"personal-digest"`
	Teams          map[string]Team           `yaml:"teams"`
}

func ParseTeamConfig(path string) (map[string]Team, []PersonalDigestUserEntry, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var tf teamsFile
	if err := yaml.NewDecoder(file).Decode(&tf); err != nil {
		return nil, nil, fmt.Errorf("decoding team config: %v", err)
	}

	for i := range tf.PersonalDigest {
		if err := applyPersonalDigestDefaults(&tf.PersonalDigest[i]); err != nil {
			return nil, nil, fmt.Errorf("personal-digest: user %q: %w", tf.PersonalDigest[i].Login, err)
		}
	}

	validSourceTypes := map[string]bool{
		"commits":   true,
		"pulls":     true,
		"issues":    true,
		"workflows": true,
		"releases":  true,
		"security":  true,
	}

	teams := tf.Teams
	for name, team := range teams {
		team.Name = name

		for _, s := range team.Sources {
			if !validSourceTypes[s.SourceType] {
				return nil, nil, fmt.Errorf("team %s: invalid source type %q", name, s.SourceType)
			}
		}

		if team.PullRequestDigest != nil {
			if err := validateDigestConfig(name, team.PullRequestDigest); err != nil {
				return nil, nil, err
			}
		}

		if team.SecurityDigest != nil {
			if err := validateSecurityDigestConfig(name, team.SecurityDigest); err != nil {
				return nil, nil, err
			}
		}

		flatSources := flatChannelsToSources(team.SlackChannels, team.Config)
		team.Sources = append(flatSources, team.Sources...)

		teams[name] = team
	}

	return teams, tf.PersonalDigest, nil
}

var validWeekdays = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

func validateDigestConfig(teamName string, d *DigestConfig) error {
	if d.Channel == "" {
		return fmt.Errorf("team %s: pr-digest.channel is required", teamName)
	}
	if !slices.Contains(validWeekdays, strings.ToLower(d.Day)) {
		return fmt.Errorf("team %s: pr-digest.day %q is not a valid weekday", teamName, d.Day)
	}
	if _, err := time.Parse("15:04", d.Time); err != nil {
		return fmt.Errorf("team %s: pr-digest.time %q must be in HH:MM format", teamName, d.Time)
	}
	tz := d.Timezone
	if tz == "" {
		tz = "Europe/Oslo"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("team %s: pr-digest.timezone %q is not a valid IANA timezone", teamName, d.Timezone)
	}
	return nil
}

func validateSecurityDigestConfig(teamName string, d *SecurityDigestConfig) error {
	if d.Channel == "" {
		return fmt.Errorf("team %s: security-digest.channel is required", teamName)
	}
	if !slices.Contains(validWeekdays, strings.ToLower(d.Day)) {
		return fmt.Errorf("team %s: security-digest.day %q is not a valid weekday", teamName, d.Day)
	}
	if _, err := time.Parse("15:04", d.Time); err != nil {
		return fmt.Errorf("team %s: security-digest.time %q must be in HH:MM format", teamName, d.Time)
	}
	tz := d.Timezone
	if tz == "" {
		tz = "Europe/Oslo"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("team %s: security-digest.timezone %q is not a valid IANA timezone", teamName, d.Timezone)
	}
	return nil
}

// flatChannelsToSources converts the old flat channel format into sources for backward compatibility.
func flatChannelsToSources(channels SlackChannels, cfg Config) []Source {
	var sources []Source

	if channels.Commits != "" {
		sources = append(sources, Source{
			SourceType: "commits",
			Channel:    channels.Commits,
		})
	}
	if channels.PullRequests != "" {
		sources = append(sources, Source{
			SourceType: "pulls",
			Channel:    channels.PullRequests,
			Config: SourceConfig{
				Pulls: cfg.Pulls,
			},
		})
	}
	if channels.Issues != "" {
		sources = append(sources, Source{
			SourceType: "issues",
			Channel:    channels.Issues,
		})
	}
	if channels.Workflows != "" {
		sources = append(sources, Source{
			SourceType: "workflows",
			Channel:    channels.Workflows,
			Config: SourceConfig{
				Workflows: cfg.Workflows,
			},
		})
	}
	if channels.Releases != "" {
		sources = append(sources, Source{
			SourceType: "releases",
			Channel:    channels.Releases,
		})
	}
	if channels.Security != "" {
		sources = append(sources, Source{
			SourceType: "security",
			Channel:    channels.Security,
			Config: SourceConfig{
				Security: cfg.Security,
			},
		})
	}

	return sources
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

// FetchOrgAsTeam fetches the organization as a team, hence there needs to be a team in the organization with the same name as the organization.
func (c Client) FetchOrgAsTeam(ctx context.Context, log *slog.Logger, reposBlocklist []string) error {
	bearerToken, err := c.createBearerToken()
	if err != nil {
		return fmt.Errorf("creating bearer token: %v", err)
	}

	// Ensure team exists in the database
	if err := c.db.CreateTeam(ctx, c.org); err != nil {
		return fmt.Errorf("creating team for organization %s: %v", c.org, err)
	}

	url := fmt.Sprintf("https://api.github.com/orgs/%s", c.org)
	if err := validateOrgExists(url, bearerToken); err != nil {
		return fmt.Errorf("validating organization %s: %v", c.org, err)
	}

	teamURL := fmt.Sprintf("%s/teams/%s", url, c.org)
	members, err := fetchMembers(teamURL, bearerToken)
	if err != nil {
		return fmt.Errorf("fetching members for %s: %v", c.org, err)
	}

	for _, member := range members {
		if err := sql.AddMemberToTeam(ctx, c.db, c.org, member.Login); err != nil {
			log.Error("Adding member to team", "team", c.org, "member", member.Login, "error", err)
			continue
		}
	}

	repositories, err := fetchRepositories(teamURL, bearerToken, reposBlocklist)
	if err != nil {
		return fmt.Errorf("fetching repositories for %s: %v", c.org, err)
	}

	for _, repository := range repositories {
		if err := sql.AddRepositoryToTeam(ctx, c.db, c.org, repository); err != nil {
			return fmt.Errorf("adding repository %s to org %s: %v", repository, c.org, err)
		}
	}

	if err := sql.RemoveRepositoriesNotBelongingToTeam(ctx, c.db, c.org, repositories); err != nil {
		return fmt.Errorf("cleaning up old repositories: %v", err)
	}

	log.Info("Subscribed to org", "org", c.org, "members", len(members), "repositories", len(repositories))

	return nil
}

func (c Client) FetchTeams(ctx context.Context, log *slog.Logger, reposBlocklist []string) error {
	bearerToken, err := c.createBearerToken()
	if err != nil {
		return fmt.Errorf("creating bearer token: %v", err)
	}

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

		if err := sql.RemoveRepositoriesNotBelongingToTeam(ctx, c.db, team, repositories); err != nil {
			return fmt.Errorf("cleaning up old repositories: %v", err)
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

		log.Info("Processed team", "team", team, "repositories", len(repositories), "members", len(members))
	}

	return nil
}
