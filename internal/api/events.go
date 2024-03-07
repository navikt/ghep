package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/redis/go-redis/v9"
)

const refHeadsPrefix = "refs/heads/"

func (c *Client) eventsPostHandler(w http.ResponseWriter, r *http.Request) {
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

	var team github.Team
	if event.Team == nil {
		var found bool

		team, found = findTeam(c.teams, event.Repository.Name)
		if !found {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	if err := c.handleEvent(team, event); err != nil {
		slog.Error("error handling event", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *Client) handleEvent(team github.Team, event github.Event) error {
	var payload []byte
	var err error
	var threadTimestamp string

	if strings.HasPrefix(event.Ref, refHeadsPrefix) {
		payload, err = handleCommitEvent(c.slack.CommitTmpl(), team, event)
	} else if event.Issue != nil {
		id := strconv.Itoa(event.Issue.ID)
		threadTimestamp, err = c.rdb.Get(c.ctx, id).Result()
		if err != nil && err != redis.Nil {
			slog.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		payload, err = handleIssueEvent(c.slack.IssueTmpl(), team, threadTimestamp, event)
	} else if event.PullRequest != nil {
		id := strconv.Itoa(event.PullRequest.ID)
		threadTimestamp, err = c.rdb.Get(c.ctx, id).Result()
		if err != nil && err != redis.Nil {
			slog.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		payload, err = handlePullRequestEvent(c.slack.PullRequestTmpl(), team, threadTimestamp, event)
	} else if event.Team != nil {
		index := slices.IndexFunc(c.teams, func(t github.Team) bool {
			return t.Name == event.Team.Name
		})
		if index == -1 {
			return nil
		}

		team := c.teams[index]

		payload, err = handleTeamEvent(c.slack.TeamTmpl(), &team, event)
		c.teams[index] = team
	} else {
		return fmt.Errorf("unknown event type")
	}

	if err != nil {
		return err
	}

	if payload == nil {
		return nil
	}

	ts, err := c.slack.PostMessage(payload)
	if err != nil {
		return err
	}

	if ts != "" && event.Action == "opened" {
		var id string
		if event.Issue != nil {
			id = strconv.Itoa(event.Issue.ID)
		} else if event.PullRequest != nil {
			id = strconv.Itoa(event.PullRequest.ID)
		}

		if err := c.rdb.Set(c.ctx, id, ts, 0).Err(); err != nil {
			slog.Error("error setting thread timestamp", "err", err.Error(), "id", id, "ts", ts)
		}
	}

	return nil
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

func handleIssueEvent(tmpl template.Template, team github.Team, threadTimestamp string, event github.Event) ([]byte, error) {
	if event.Action != "opened" && event.Action != "closed" {
		return nil, nil
	}

	slog.Info(fmt.Sprintf("Received issue to %v for %v (action: %v)", event.Repository.Name, team.Name, event.Action))
	return slack.CreateIssueMessage(tmpl, team.SlackChannels.Issues, threadTimestamp, event)
}

func handlePullRequestEvent(tmpl template.Template, team github.Team, threadTimestamp string, event github.Event) ([]byte, error) {
	if event.Action != "opened" && event.Action != "closed" {
		return nil, nil
	}

	slog.Info(fmt.Sprintf("Received pull request to %v for %v (action: %v)", event.Repository.Name, team.Name, event.Action))
	return slack.CreatePullRequestMessage(tmpl, team.SlackChannels.PullRequests, threadTimestamp, event)
}

func handleTeamEvent(tmpl template.Template, team *github.Team, event github.Event) ([]byte, error) {
	if event.Action != "added_to_repository" && event.Action != "removed_from_repository" {
		return nil, nil
	}

	slog.Info(fmt.Sprintf("Received team event to %v for %v (action: %v)", event.Repository.Name, team.Name, event.Action))
	if event.Action == "added_to_repository" {
		team.Repositories = append(team.Repositories, event.Repository.Name)
	} else {
		for i, repo := range team.Repositories {
			if repo == event.Repository.Name {
				team.Repositories = append(team.Repositories[:i], team.Repositories[i+1:]...)
				break
			}
		}
	}

	return slack.CreateTeamMessage(tmpl, team.SlackChannels.Commits, event)
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
