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

type simpleEvent struct {
	Refs       string `json:"refs"`
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

func (c client) events(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("error reading body", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	simpleEvent := simpleEvent{}
	if err := json.Unmarshal(body, &simpleEvent); err != nil {
		slog.Error("error decoding body", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if simpleEvent.Zen != "" {
		slog.Info("Received ping event")
		w.WriteHeader(http.StatusOK)
		return
	}

	if strings.HasPrefix(simpleEvent.Refs, "refs/tags/") {
		slog.Info("Received commit event")
		branch := strings.TrimPrefix(simpleEvent.Refs, "refs/heads/")
		if err := c.handleCommitEvent(body, simpleEvent.Repository.Name, branch); err != nil {
			slog.Error("error handling commit event", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
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
