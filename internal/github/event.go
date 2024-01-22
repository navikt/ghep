package github

import (
	"encoding/json"
	"fmt"
)

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Commit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	URL     string `json:"url"`
}

type Repository struct {
	Name string `json:"name"`
	URL  string `json:"html_url"`
}

type CommitEvent struct {
	Commits    []Commit   `json:"commits"`
	Repository Repository `json:"repository"`
	Compare    string     `json:"compare"`
	Pusher     Author     `json:"pusher"`
}

func CreateCommitEvent(body []byte) (CommitEvent, error) {
	event := CommitEvent{}
	if err := json.Unmarshal(body, &event); err != nil {
		return CommitEvent{}, fmt.Errorf("decoding commit event: %w", err)
	}

	return event, nil
}
