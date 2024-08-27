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
	"time"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/redis/go-redis/v9"
)

const (
	refHeadsPrefix = "refs/heads/"
	oneYear        = 8760 * time.Hour
)

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

	log := slog.With("repository", event.Repository.Name, "team", team.Name, "action", event.Action)
	if err := c.handleEvent(log, team, event); err != nil {
		log.Error("error handling event", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *Client) handleEvent(log *slog.Logger, team github.Team, event github.Event) error {
	var payload []byte
	var err error
	var threadTimestamp string

	if strings.HasPrefix(event.Ref, refHeadsPrefix) {
		payload, err = handleCommitEvent(log, c.slack.CommitTmpl(), team, event)
	} else if event.Issue != nil {
		id := strconv.Itoa(event.Issue.ID)
		threadTimestamp, err = c.rdb.Get(c.ctx, id).Result()
		if err != nil && err != redis.Nil {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		payload, err = handleIssueEvent(log, c.slack.IssueTmpl(), team, threadTimestamp, event)
	} else if event.PullRequest != nil {
		id := strconv.Itoa(event.PullRequest.ID)
		threadTimestamp, err = c.rdb.Get(c.ctx, id).Result()
		if err != nil && err != redis.Nil {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		payload, err = handlePullRequestEvent(log, c.slack.PullRequestTmpl(), team, threadTimestamp, event)
	} else if event.Team != nil {
		index := slices.IndexFunc(c.teams, func(t github.Team) bool {
			return t.Name == event.Team.Name
		})
		if index == -1 {
			return nil
		}

		team := c.teams[index]

		payload, err = handleTeamEvent(log, c.slack.TeamTmpl(), &team, event)
		c.teams[index] = team
	} else if event.Workflow != nil {
		var timestamp string
		id := event.Workflow.HeadSHA
		timestamp, err = c.rdb.Get(c.ctx, id).Result()
		if err != nil && err != redis.Nil {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		if timestamp != "" {
			if err := c.slack.PostWorkflowReaction(log, team, event, timestamp); err != nil {
				log.Error("error posting workflow reaction", "err", err.Error())
			}
		}

		payload, err = handleWorkflowEvent(log, c.slack.WorkflowTmpl(), team, event)
	} else {
		log.Info("unknown event type")
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

	if ts != "" && saveEventSlackResponse(event) {
		var id string
		if event.Issue != nil {
			id = strconv.Itoa(event.Issue.ID)
		} else if event.PullRequest != nil {
			id = strconv.Itoa(event.PullRequest.ID)
		} else if strings.HasPrefix(event.Ref, refHeadsPrefix) {
			id = event.After
		} else {
			return fmt.Errorf("unknown id string when saving timestamp")
		}

		if err := c.rdb.Set(c.ctx, id, ts, oneYear).Err(); err != nil {
			log.Error("error setting thread timestamp", "err", err.Error(), "id", id, "ts", ts)
		}
	}

	return nil
}

func saveEventSlackResponse(event github.Event) bool {
	if event.Action != "opened" {
		return event.Issue != nil || event.PullRequest != nil
	} else if strings.HasPrefix(event.Ref, refHeadsPrefix) {
		return true
	}

	return false
}

func handleCommitEvent(log *slog.Logger, tmpl template.Template, team github.Team, event github.Event) ([]byte, error) {
	branch := strings.TrimPrefix(event.Ref, refHeadsPrefix)

	if team.SlackChannels.Commits == "" {
		return nil, nil
	}

	if branch != event.Repository.DefaultBranch {
		return nil, nil
	}

	if len(event.Commits) == 0 {
		return nil, nil
	}

	log.Info("Received commit event", "slack_channel", team.SlackChannels.Commits)
	return slack.CreateCommitMessage(tmpl, team.SlackChannels.Commits, event)
}

func handleIssueEvent(log *slog.Logger, tmpl template.Template, team github.Team, threadTimestamp string, event github.Event) ([]byte, error) {
	if team.SlackChannels.Issues == "" {
		return nil, nil
	}

	if event.Action != "opened" && event.Action != "closed" {
		return nil, nil
	}

	log.Info("Received issue", "slack_channel", team.SlackChannels.Issues)
	return slack.CreateIssueMessage(tmpl, team.SlackChannels.Issues, threadTimestamp, event)
}

func handlePullRequestEvent(log *slog.Logger, tmpl template.Template, team github.Team, threadTimestamp string, event github.Event) ([]byte, error) {
	if team.SlackChannels.PullRequests == "" {
		return nil, nil
	}

	if event.Action != "opened" && event.Action != "closed" && event.Action != "reopened" {
		return nil, nil
	}

	channel := team.SlackChannels.PullRequests
	if team.Config.ExternalContributorsChannel != "" && !team.IsMember(event.Sender.Login) {
		channel = team.Config.ExternalContributorsChannel
	}

	log.Info("Received pull request", "slack_channel", channel)
	return slack.CreatePullRequestMessage(tmpl, channel, threadTimestamp, event)
}

func handleTeamEvent(log *slog.Logger, tmpl template.Template, team *github.Team, event github.Event) ([]byte, error) {
	if team.SlackChannels.Commits == "" {
		return nil, nil
	}

	if event.Action != "added_to_repository" && event.Action != "removed_from_repository" {
		return nil, nil
	}

	log.Info("Received team event", "slack_channel", team.SlackChannels.Commits)
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

func handleWorkflowEvent(log *slog.Logger, tmpl template.Template, team github.Team, event github.Event) ([]byte, error) {
	if team.SlackChannels.Workflows == "" {
		return nil, nil
	}

	if team.Config.Workflows.IgnoreBots && event.Sender.Type == "Bot" {
		return nil, nil
	}

	if len(team.Config.Workflows.Branches) > 0 && !slices.Contains(team.Config.Workflows.Branches, event.Workflow.HeadBranch) {
		return nil, nil
	}

	if event.Action != "completed" || event.Workflow.Conclusion != "failure" {
		return nil, nil
	}

	log.Info("Received workflow run", "conclusion", event.Workflow.Conclusion, "slack_channel", team.SlackChannels.Workflows)
	return slack.CreateWorkflowMessage(tmpl, team.SlackChannels.Workflows, event)
}
