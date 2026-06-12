package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"
)

// SecurityAlert represents a single open security alert from any of the three alert types.
type SecurityAlert struct {
	AlertType  SecurityAlertType
	SecretType string
	// RuleDescription is the rule description for code scanning alerts.
	RuleDescription string
	// Severity is the severity string for code scanning and Dependabot alerts.
	Severity string
	// AdvisorySummary is the advisory summary for Dependabot alerts.
	AdvisorySummary string
}

type SecurityAlertType int

const (
	AlertTypeSecretScanning SecurityAlertType = iota
	AlertTypeCodeScanning
	AlertTypeDependabot
)

type RepoSecurityAlerts struct {
	Repository     Repository
	SecretScanning []SecurityAlert
	CodeScanning   []SecurityAlert
	Dependabot     []SecurityAlert
}

func (r RepoSecurityAlerts) ToSlack() string {
	return fmt.Sprintf("<%s/security|%s>", r.Repository.URL, r.Repository.Name)
}

func (r RepoSecurityAlerts) ToSlackWithMetadata(path string, alerts int) string {
	return fmt.Sprintf("<%s/security/%s|%d>", r.Repository.URL, path, alerts)
}

func (r RepoSecurityAlerts) Total() int {
	return len(r.SecretScanning) + len(r.CodeScanning) + len(r.Dependabot)
}

func (c Client) FetchOpenSecurityAlerts(ctx context.Context, teamSlug string, cfg *SecurityDigestConfig, globalIgnore []string) ([]RepoSecurityAlerts, error) {
	bearerToken, err := c.createBearerToken()
	if err != nil {
		return nil, fmt.Errorf("creating bearer token: %v", err)
	}

	repos, err := c.db.ListTeamRepositories(ctx, teamSlug)
	if err != nil {
		return nil, fmt.Errorf("listing repositories for team %s: %v", teamSlug, err)
	}
	if len(repos) == 0 {
		return nil, nil
	}

	// Build set of repos for this team, excluding ignored repos from both config levels.
	repoSet := make(map[string]bool, len(repos))
	for _, r := range repos {
		if slices.Contains(globalIgnore, r.Name) {
			continue
		}
		if slices.Contains(cfg.IgnoreRepositories, r.Name) {
			continue
		}
		repoSet[r.Name] = true
	}

	httpClient := http.Client{Timeout: 30 * time.Second}

	secretAlerts, err := fetchOrgSecretScanningAlerts(ctx, httpClient, bearerToken, c.org)
	if err != nil {
		return nil, fmt.Errorf("fetching secret scanning alerts: %v", err)
	}

	codeScanningAlerts, err := fetchOrgCodeScanningAlerts(ctx, httpClient, bearerToken, c.org)
	if err != nil {
		return nil, fmt.Errorf("fetching code scanning alerts: %v", err)
	}

	dependabotAlerts, err := fetchOrgDependabotAlerts(ctx, httpClient, bearerToken, c.org)
	if err != nil {
		return nil, fmt.Errorf("fetching dependabot alerts: %v", err)
	}

	// Group by repo, filtering to team's repos.
	byRepo := make(map[string]*RepoSecurityAlerts)

	for _, a := range secretAlerts {
		if !repoSet[a.repo.Name] {
			continue
		}
		entry := getOrCreate(byRepo, a.repo)
		entry.SecretScanning = append(entry.SecretScanning, SecurityAlert{
			AlertType:  AlertTypeSecretScanning,
			SecretType: a.secretType,
		})
	}

	threshold := cfg.SeverityType()

	for _, a := range codeScanningAlerts {
		if !repoSet[a.repo.Name] {
			continue
		}
		if AsSeverityType(a.severity) < threshold {
			continue
		}
		entry := getOrCreate(byRepo, a.repo)
		entry.CodeScanning = append(entry.CodeScanning, SecurityAlert{
			AlertType:       AlertTypeCodeScanning,
			RuleDescription: a.ruleDescription,
			Severity:        a.severity,
		})
	}

	for _, a := range dependabotAlerts {
		if !repoSet[a.repo.Name] {
			continue
		}
		if AsSeverityType(a.severity) < threshold {
			continue
		}
		entry := getOrCreate(byRepo, a.repo)
		entry.Dependabot = append(entry.Dependabot, SecurityAlert{
			AlertType:       AlertTypeDependabot,
			AdvisorySummary: a.advisorySummary,
			Severity:        a.severity,
		})
	}

	result := make([]RepoSecurityAlerts, 0, len(byRepo))
	for _, v := range byRepo {
		result = append(result, *v)
	}
	return result, nil
}

func getOrCreate(m map[string]*RepoSecurityAlerts, repository Repository) *RepoSecurityAlerts {
	if v, ok := m[repository.Name]; ok {
		return v
	}

	v := &RepoSecurityAlerts{Repository: repository}
	m[repository.Name] = v
	return v
}

// --- internal REST response types ---
type orgSecretAlert struct {
	repo       Repository
	url        string
	secretType string
}

type orgCodeScanningAlert struct {
	repo            Repository
	ruleDescription string
	severity        string
}

type orgDependabotAlert struct {
	repo            Repository
	advisorySummary string
	severity        string
}

func fetchOrgSecretScanningAlerts(ctx context.Context, httpClient http.Client, bearerToken, org string) ([]orgSecretAlert, error) {
	type apiAlert struct {
		SecretTypeDisplay string     `json:"secret_type_display_name"`
		Repository        Repository `json:"repository"`
	}

	raw, err := fetchAllPages[apiAlert](ctx, httpClient, bearerToken,
		fmt.Sprintf("https://api.github.com/orgs/%s/secret-scanning/alerts?state=open&per_page=100", org))
	if err != nil {
		return nil, err
	}

	result := make([]orgSecretAlert, 0, len(raw))
	for _, a := range raw {
		result = append(result, orgSecretAlert{
			repo:       a.Repository,
			url:        a.Repository.URL,
			secretType: a.SecretTypeDisplay,
		})
	}
	return result, nil
}

func fetchOrgCodeScanningAlerts(ctx context.Context, httpClient http.Client, bearerToken, org string) ([]orgCodeScanningAlert, error) {
	type apiAlert struct {
		Rule struct {
			Description           string `json:"description"`
			SecuritySeverityLevel string `json:"security_severity_level"`
		} `json:"rule"`
		Repository Repository `json:"repository"`
	}

	raw, err := fetchAllPages[apiAlert](ctx, httpClient, bearerToken,
		fmt.Sprintf("https://api.github.com/orgs/%s/code-scanning/alerts?state=open&per_page=100", org))
	if err != nil {
		return nil, err
	}

	result := make([]orgCodeScanningAlert, 0, len(raw))
	for _, a := range raw {
		result = append(result, orgCodeScanningAlert{
			repo:            a.Repository,
			ruleDescription: a.Rule.Description,
			severity:        a.Rule.SecuritySeverityLevel,
		})
	}
	return result, nil
}

func fetchOrgDependabotAlerts(ctx context.Context, httpClient http.Client, bearerToken, org string) ([]orgDependabotAlert, error) {
	type apiAlert struct {
		SecurityAdvisory struct {
			Summary  string `json:"summary"`
			Severity string `json:"severity"`
		} `json:"security_advisory"`
		Repository Repository `json:"repository"`
	}

	raw, err := fetchAllPages[apiAlert](ctx, httpClient, bearerToken,
		fmt.Sprintf("https://api.github.com/orgs/%s/dependabot/alerts?state=open&per_page=100", org))
	if err != nil {
		return nil, err
	}

	result := make([]orgDependabotAlert, 0, len(raw))
	for _, a := range raw {
		result = append(result, orgDependabotAlert{
			repo:            a.Repository,
			advisorySummary: a.SecurityAdvisory.Summary,
			severity:        a.SecurityAdvisory.Severity,
		})
	}
	return result, nil
}

func fetchAllPages[T any](ctx context.Context, httpClient http.Client, bearerToken, url string) ([]T, error) {
	var result []T
	next := url

	for next != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", next, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Add("Authorization", "Bearer "+bearerToken)
		req.Header.Add("Content-Type", "application/json")

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
				return nil, fmt.Errorf("request failed (%v): %s", resp.Status, body)
			}
			return nil, fmt.Errorf("request failed (%v): %s (%s)", resp.Status, ghErr.Message, ghErr.Status)
		}

		var page []T
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("unmarshalling response: %v", err)
		}
		result = append(result, page...)

		next = nextPageURL(resp.Header.Get("Link"))
	}

	return result, nil
}

func nextPageURL(link string) string {
	if link == "" {
		return ""
	}
	// Link header format: <url>; rel="next", <url>; rel="last"
	for _, part := range splitLink(link) {
		var url, rel string
		for i, segment := range splitSemicolon(part) {
			s := trimSpace(segment)
			if i == 0 && len(s) >= 2 && s[0] == '<' && s[len(s)-1] == '>' {
				url = s[1 : len(s)-1]
			} else if s == `rel="next"` {
				rel = "next"
			}
		}
		if rel == "next" {
			return url
		}
	}
	return ""
}

func splitLink(s string) []string      { return splitOn(s, ',') }
func splitSemicolon(s string) []string { return splitOn(s, ';') }

func splitOn(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
