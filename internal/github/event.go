package github

import (
	"encoding/json"
	"fmt"
)

type Sender struct {
	Login string `json:"login"`
	URL   string `json:"html_url"`
}

type Repository struct {
	Name          string `json:"name"`
	URL           string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
	RoleName      string `json:"role_name"`
}

type Commit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	URL     string `json:"url"`
}

// Issue is a struct for issues and pull requests
// Every pull request is an issue, but not every issue is a pull request
type Issue struct {
	Action      string `json:"action"`
	ID          int    `json:"id"`
	URL         string `json:"html_url"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Number      int    `json:"number"`
	StateReason string `json:"state_reason"`
	Merged      bool   `json:"merged"`
}

type TeamEvent struct {
	Name string `json:"name"`
	URL  string `json:"html_url"`
}

type Event struct {
	Action      string     `json:"action"`
	Ref         string     `json:"ref"`
	Repository  Repository `json:"repository"`
	Commits     []Commit   `json:"commits"`
	Compare     string     `json:"compare"`
	Issue       *Issue     `json:"issue"`
	PullRequest *Issue     `json:"pull_request"`
	Sender      Sender     `json:"sender"`
	Team        *TeamEvent `json:"team"`
}

func CreateEvent(body []byte) (Event, error) {
	event := Event{}
	if err := json.Unmarshal(body, &event); err != nil {
		return Event{}, fmt.Errorf("decoding event: %w", err)
	}

	return event, nil
}
