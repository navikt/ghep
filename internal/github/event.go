package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type User struct {
	Name  string
	Login string `json:"login"`
	Type  string `json:"type"`
	URL   string `json:"html_url"`
}

func (u User) ToSlack() string {
	if u.URL == "" {
		return u.Login
	}

	return fmt.Sprintf("<%s|%s>", u.URL, u.Login)
}

func (u User) IsDependabot() bool {
	return u.Login == "dependabot[bot]" || u.Login == "github-actions[bot]"
}

type Author struct {
	Name     string `json:"name"`
	Username string `json:"username"`
}

func (a Author) AsUser() User {
	return User{
		Name:  a.Name,
		Login: a.Username,
		Type:  "User",
		URL:   "https://github.com/" + a.Username,
	}
}

type Changes struct {
	Repository struct {
		Name struct {
			From string `json:"from"`
		} `json:"name"`
	} `json:"repository"`
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
}

type TeamEvent struct {
	Name string `json:"name"`
	URL  string `json:"html_url"`
}

type FailedJob struct {
	Name string
	URL  string
	Step string
}

type Workflow struct {
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

type Event struct {
	Action              string       `json:"action"`
	Ref                 string       `json:"ref"`
	After               string       `json:"after"`
	Repository          Repository   `json:"repository"`
	Changes             Changes      `json:"changes"`
	Commits             []Commit     `json:"commits"`
	Compare             string       `json:"compare"`
	Issue               *Issue       `json:"issue"`
	PullRequest         *Issue       `json:"pull_request"`
	RepositoriesRemoved []Repository `json:"repositories_removed"`
	Sender              User         `json:"sender"`
	User                User         `json:"user"`
	Team                *TeamEvent   `json:"team"`
	Workflow            *Workflow    `json:"workflow_run"`
}

func (e Event) FindRepositoryName() string {
	switch e.Action {
	case "renamed":
		return e.Changes.Repository.Name.From
	case "removed":
		return e.RepositoriesRemoved[0].Name
	default:
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
