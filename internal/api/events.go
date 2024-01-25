package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

const refHeadsPrefix = "refs/heads/"

type simpleEvent struct {
	Ref        string `json:"ref"`
	Repository struct {
		Name string `json:"name"`
	} `json:"repository"`
	Zen string `json:"zen"`
}

func findTeam(teams []github.Team, repositoryName string) (github.Team, bool) {
	for _, team := range teams {
		for _, repo := range team.Repositories {
			if repo == repositoryName {
				return team, true
			}
		}
	}

	return github.Team{}, false
}

func (c client) eventsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("error reading body", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	event := simpleEvent{}
	if err := json.Unmarshal(body, &event); err != nil {
		slog.Error("error decoding body", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := c.handleSimpleEvent(body, event); err != nil {
		slog.Error("error handling simple event", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c client) handleSimpleEvent(body []byte, event simpleEvent) error {
	if strings.HasPrefix(event.Ref, refHeadsPrefix) {
		branch := strings.TrimPrefix(event.Ref, refHeadsPrefix)
		return c.handleCommitEvent(body, event.Repository.Name, branch)
	}

	return nil
}

func (c client) handleCommitEvent(body []byte, repository, branch string) error {
	team, ok := findTeam(c.teams, repository)
	if !ok {
		return fmt.Errorf("no team found for repository %v", repository)
	}

	slog.Info(fmt.Sprintf("Received commit to %v for %v", repository, team.Name))
	commit, err := github.CreateCommitEvent(body)
	if err != nil {
		return fmt.Errorf("error creating commit event: %v", err.Error())
	}

	if commit.Repository.DefaultBranch != branch {
		slog.Info(fmt.Sprintf("Ignoring commit to %v on branch %v", repository, branch))
		return nil
	}

	if len(commit.Commits) == 0 {
		slog.Info("No commits to process")
		return nil
	}

	payload, err := slack.CreateCommitMessage(team.SlackChannels.Commits, commit)
	if err != nil {
		return fmt.Errorf("error creating Slack message: %v", err.Error())
	}

	if err := c.slack.PostMessage(payload); err != nil {
		return fmt.Errorf("error posting to Slack: %v", err.Error())
	}

	slog.Info("Successfully posted to Slack")
	return nil
}
