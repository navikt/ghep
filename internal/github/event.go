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
	Id        string `json:"id"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Url       string `json:"url"`
	Author    Author `json:"author"`
}

type Repository struct {
	Name    string `json:"name"`
	HtmlUrl string `json:"html_url"`
}

type CommitEvent struct {
	Commits    []Commit   `json:"commits"`
	Repository Repository `json:"repository"`
}

func CreateCommitEvent(body []byte) (CommitEvent, error) {
	event := CommitEvent{}
	if err := json.Unmarshal(body, &event); err != nil {
		return CommitEvent{}, fmt.Errorf("decoding commit event: %w", err)
	}

	return event, nil
}
