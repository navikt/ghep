package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"text/template"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

const refHeadsPrefix = "refs/heads/"

func (c Client) eventsHandler(w http.ResponseWriter, r *http.Request) {
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

	event, err := github.CreateEvent(body)
	if err != nil {
		slog.Error("error creating event", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	team, found := findTeam(c.teams, event.Repository.Name)
	if !found {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := c.handleEvent(team, event); err != nil {
		slog.Error("error handling event", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c Client) handleEvent(team github.Team, event github.Event) error {
	var payload []byte
	var err error

	if strings.HasPrefix(event.Ref, refHeadsPrefix) {
		payload, err = handleCommitEvent(c.slack.CommitTmpl(), team, event)
	} else if event.Issue != nil {
		payload, err = handleIssueEvent(c.slack.IssueTmpl(), team, event)
	}

	if err != nil {
		return err
	}

	if payload == nil {
		return nil
	}

	return c.slack.PostMessage(payload)
}

func handleCommitEvent(tmpl template.Template, team github.Team, event github.Event) ([]byte, error) {
	branch := strings.TrimPrefix(event.Ref, refHeadsPrefix)

	if branch != event.Repository.DefaultBranch {
		return nil, nil
	}

	if len(event.Commits) == 0 {
		return nil, nil
	}

	slog.Info(fmt.Sprintf("Received commit to %v for %v", event.Repository.Name, team.Name))
	return slack.CreateCommitMessage(tmpl, team.SlackChannels.Commits, event)
}

func handleIssueEvent(tmpl template.Template, team github.Team, event github.Event) ([]byte, error) {
	slog.Info(fmt.Sprintf("Received issue to %v for %v", event.Repository.Name, team.Name))
	return slack.CreateIssueMessage(tmpl, team.SlackChannels.Issues, event)
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
