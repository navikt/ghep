//go:generate stringer -type=EventType

package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type (
	EventType    int
	SeverityType int
)

const (
	RefHeadsPrefix = "refs/heads/"

	TypeCommit EventType = iota
	TypeCodeScanningAlert
	TypeDependabotAlert
	TypeIssue
	TypePullRequest
	TypeRelease
	TypeRepositoryRenamed
	TypeRepositoryPublic
	TypeSecurityAdvisory
	TypeSecretScanningAlert
	TypeTeam
	TypeWorkflow
	TypeUnknown

	SeverityLow SeverityType = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

type User struct {
	Login string `json:"login"`
	Type  string `json:"type"`
	URL   string `json:"html_url"`
}

// ToSlack returns a formatted string for Slack
// If the URL is empty, it returns the login
// If the URL is not empty, it returns <URL|Login>
func (u User) ToSlack() string {
	if u.URL == "" {
		return u.Login
	}

	return fmt.Sprintf("<%s|%s>", u.URL, u.Login)
}

func (u User) IsBot() bool {
	return u.Type == "Bot" || isBot(u.Login)
}

func (u User) IsUser() bool {
	return u.Type == "User"
}

type Author struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

func (a Author) IsBot() bool {
	return isBot(a.Username)
}

func isBot(username string) bool {
	return strings.HasSuffix(username, "[bot]") || strings.HasSuffix(username, "bot")
}

func (a Author) AsUser() User {
	var user User

	if a.IsBot() {
		user.Login = a.Username
		user.Type = "Bot"
		user.URL = "https://github.com/apps/" + strings.TrimSuffix(a.Username, "[bot]")
	} else {
		user.Login = a.Username
		user.Type = "User"

		if a.Username == "" {
			user.Login = a.Name
		} else {
			user.URL = "https://github.com/" + a.Username
		}
	}

	return user
}

type Changes struct {
	Repository struct {
		Name struct {
			From string `json:"from"`
		} `json:"name"`
	} `json:"repository"`
}

// ToSlack returns a formatted string for Slack
// If the URL is empty, it returns the repository name
// If the URL is not empty, it returns <URL|Name>
func (r Repository) ToSlack() string {
	if r.URL == "" {
		return r.Name
	}

	return fmt.Sprintf("<%s|%s>", r.URL, r.Name)
}

type Repository struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	URL           string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
	RoleName      string `json:"role_name"`
}

type Commit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	URL     string `json:"url"`
	Author  Author `json:"author"`
}

type Rule struct {
	Description           string `json:"description"`
	FullDescription       string `json:"full_description"`
	SecuritySeverityLevel string `json:"security_severity_level"`
}

func (r Rule) SeverityType() SeverityType {
	return AsSeverityType(r.SecuritySeverityLevel)
}

func AsSeverityType(level string) SeverityType {
	switch level {
	case "medium", "moderate":
		return SeverityMedium
	case "high":
		return SeverityHigh
	case "critical":
		return SeverityCritical
	default:
		return SeverityCritical
	}
}

type Tool struct {
	Name string `json:"name"`
}

type Alert struct {
	PubliclyLeaked    bool              `json:"publicly_leaked"`
	SecretType        *string           `json:"secret_type_display_name"`
	State             string            `json:"state"`
	URL               string            `json:"html_url"`
	Resolution        string            `json:"resolution"`
	ResolutionComment string            `json:"resolution_comment"`
	ResolvedBy        User              `json:"resolved_by"`
	Rule              Rule              `json:"rule"`
	Tool              *Tool             `json:"tool"`
	SecurityAdvisory  *SecurityAdvisory `json:"security_advisory"`
}

// Issue is a struct for issues and pull requests
// Every pull request is an issue, but not every issue is a pull request
type Issue struct {
	Action      string `json:"action"`
	Draft       bool   `json:"draft"`
	ID          int    `json:"id"`
	URL         string `json:"html_url"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Number      int    `json:"number"`
	State       string `json:"state"`
	StateReason string `json:"state_reason"`
	Merged      bool   `json:"merged"`
	User        User   `json:"user"`
}

type Release struct {
	ID         int    `json:"id"`
	URL        string `json:"html_url"`
	User       User   `json:"author"`
	Tag        string `json:"tag_name"`
	Name       string `json:"name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Body       string `json:"body"`
}

type TeamEvent struct {
	Name string `json:"name"`
	URL  string `json:"html_url"`
}

// ToSlack returns a formatted string for Slack
// If the URL is empty, it returns the repository name
// If the URL is not empty, it returns <URL|Name>
func (t TeamEvent) ToSlack() string {
	if t.URL == "" {
		return t.Name
	}

	return fmt.Sprintf("<%s|%s>", t.URL, t.Name)
}

type Membership struct {
	User User `json:"user"`
}

type SecurityAdvisory struct {
	CVEID       string `json:"cve_id"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	References  []struct {
		URL string `json:"url"`
	} `json:"references"`
}

func (s SecurityAdvisory) SeverityType() SeverityType {
	return AsSeverityType(s.Severity)
}

type FailedJob struct {
	Name string
	URL  string
	Step string
}

type Workflow struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	HeadBranch string `json:"head_branch"`
	HeadSHA    string `json:"head_sha"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Title      string `json:"display_title"`
	RunNumber  int    `json:"run_number"`
	URL        string `json:"html_url"`
	JobsURL    string `json:"jobs_url"`
	FailedJob  FailedJob
}

// UpdateFailedJob finds and update the failed job in a workflow
func (w *Workflow) UpdateFailedJob() error {
	type Workflow struct {
		Jobs []struct {
			Name       string `json:"name"`
			URL        string `json:"html_url"`
			Conclusion string `json:"conclusion"`
			Steps      []struct {
				Name        string `json:"name"`
				Conclusions string `json:"conclusion"`
			} `json:"steps"`
		} `json:"jobs"`
	}

	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := httpClient.Get(w.JobsURL)
	if err != nil {
		return fmt.Errorf("getting jobs: %w", err)
	}

	defer resp.Body.Close()

	var jobs Workflow
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return fmt.Errorf("decoding jobs: %w", err)
	}

	for _, job := range jobs.Jobs {
		if job.Conclusion == "failure" {
			for _, step := range job.Steps {
				if step.Conclusions == "failure" {
					w.FailedJob = FailedJob{
						Name: job.Name,
						URL:  job.URL,
						Step: step.Name,
					}

					return nil
				}
			}
		}
	}

	return nil
}

func (e Event) IsCommit() bool {
	return strings.HasPrefix(e.Ref, RefHeadsPrefix) && e.Alert == nil
}

type Event struct {
	Action              string            `json:"action"`
	Alert               *Alert            `json:"alert"`
	Ref                 string            `json:"ref"`
	After               string            `json:"after"`
	Repository          *Repository       `json:"repository"`
	Changes             *Changes          `json:"changes"`
	Commits             []Commit          `json:"commits"`
	Compare             string            `json:"compare"`
	Issue               *Issue            `json:"issue"`
	PullRequest         *Issue            `json:"pull_request"`
	Release             *Release          `json:"release"`
	RepositoriesRemoved []Repository      `json:"repositories_removed"`
	Sender              User              `json:"sender"`
	Team                *TeamEvent        `json:"team"`
	Member              User              `json:"member"`
	Membership          Membership        `json:"membership"`
	SecurityAdvisory    *SecurityAdvisory `json:"security_advisory"`
	Workflow            *Workflow         `json:"workflow_run"`
}

func (e Event) GetEventType() EventType {
	if e.IsCommit() {
		return TypeCommit
	} else if e.Alert != nil {
		if e.Alert.SecretType != nil {
			return TypeSecretScanningAlert
		} else if e.Alert.Tool != nil {
			return TypeCodeScanningAlert
		} else if e.Alert.SecurityAdvisory != nil {
			return TypeDependabotAlert
		}
	} else if e.SecurityAdvisory != nil {
		return TypeSecurityAdvisory
	} else if e.Issue != nil {
		return TypeIssue
	} else if e.PullRequest != nil {
		return TypePullRequest
	} else if e.Release != nil {
		return TypeRelease
	} else if e.Changes != nil && e.Action == "renamed" {
		return TypeRepositoryRenamed
	} else if e.Repository != nil && e.Action == "publicized" {
		return TypeRepositoryPublic
	} else if e.Team != nil {
		return TypeTeam
	} else if e.Workflow != nil {
		return TypeWorkflow
	}

	return TypeUnknown
}

func (e Event) GetRepositoryName() string {
	if e.Action == "renamed" && e.Changes != nil {
		return e.Changes.Repository.Name.From
	} else if e.Action == "removed" && len(e.RepositoriesRemoved) > 0 {
		return e.RepositoriesRemoved[0].Name
	} else {
		if e.Repository == nil {
			return ""
		}

		return e.Repository.Name
	}
}

func CreateEvent(body []byte) (Event, error) {
	event := Event{}
	if err := json.Unmarshal(body, &event); err != nil {
		return Event{}, fmt.Errorf("decoding event: %w", err)
	}

	return event, nil
}
