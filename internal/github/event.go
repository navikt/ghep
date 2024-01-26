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
}

type Commit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	URL     string `json:"url"`
}

type Event struct {
	Action     string     `json:"action"`
	Ref        string     `json:"ref"`
	Repository Repository `json:"repository"`
	Commits    []Commit   `json:"commits"`
	Compare    string     `json:"compare"`
	Sender     Sender     `json:"sender"`
}

func CreateEvent(body []byte) (Event, error) {
	event := Event{}
	if err := json.Unmarshal(body, &event); err != nil {
		return Event{}, fmt.Errorf("decoding event: %w", err)
	}

	return event, nil
}
