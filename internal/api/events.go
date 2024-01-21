package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

type simpleEvent struct {
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
	slog.Info("Received event")
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("error reading body", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

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

	team, ok := findTeam(c.teams, simpleEvent.Repository.Name)
	if !ok {
		w.WriteHeader(http.StatusOK)
		return
	}

	slog.Info(fmt.Sprintf("Received repository event for v%:%v", team.Name, simpleEvent.Repository.Name))
	if err := c.processTeam(team, body); err != nil {
		slog.Error("error processing team", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c client) processTeam(team github.Team, body []byte) error {
	// TODO: Check what kind of event this is

	commit, err := github.CreateCommitEvent(body)
	if err != nil {
		return fmt.Errorf("error creating commit event: %v", err.Error())
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
