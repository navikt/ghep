package events

import (
	"context"
	"fmt"
	"log/slog"
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

type Handler struct {
	github github.Client
	redis  *redis.Client
	slack  slack.Client
	teams  []github.Team
}

func NewHandler(githubClient github.Client, redis *redis.Client, slackClient slack.Client, teams []github.Team) Handler {
	return Handler{
		github: githubClient,
		redis:  redis,
		slack:  slackClient,
		teams:  teams,
	}
}

func (h *Handler) Handle(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) error {
	payload, err := h.handle(ctx, log, team, event)
	if err != nil {
		return err
	}

	if payload == nil {
		return nil
	}

	ts, err := h.slack.PostMessage(payload)
	if err != nil {
		return err
	}

	if saveEventSlackResponse(ts, event) {
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

		if err := h.redis.Set(ctx, id, ts, oneYear).Err(); err != nil {
			log.Error("error setting thread timestamp", "err", err.Error(), "id", id, "ts", ts)
		}
	}

	return nil
}

func saveEventSlackResponse(ts string, event github.Event) bool {
	if ts != "" {
		return true
	} else if event.Action != "opened" {
		return event.Issue != nil || event.PullRequest != nil
	} else if strings.HasPrefix(event.Ref, refHeadsPrefix) {
		return true
	}

	return false
}

func (h Handler) handle(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) ([]byte, error) {
	if strings.HasPrefix(event.Ref, refHeadsPrefix) {
		return handleCommitEvent(log, h.slack.CommitTmpl(), team, event, h.github)
	} else if event.Issue != nil {
		id := strconv.Itoa(event.Issue.ID)
		threadTimestamp, err := h.redis.Get(ctx, id).Result()
		if err != nil && err != redis.Nil {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		return handleIssueEvent(log, h.slack.IssueTmpl(), team, threadTimestamp, event)
	} else if event.PullRequest != nil {
		id := strconv.Itoa(event.PullRequest.ID)
		threadTimestamp, err := h.redis.Get(ctx, id).Result()
		if err != nil && err != redis.Nil {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		return handlePullRequestEvent(log, h.slack.PullRequestTmpl(), team, threadTimestamp, event)
	} else if event.Action == "removed" {
		return handleRemoveRepositoryEvent(log, h.slack.RemovedTmpl(), &team, event)
	} else if event.Action == "renamed" {
		return handleRenamedRepository(log, h.slack.RenamedTmpl(), &team, event)
	} else if event.Team != nil {
		index := slices.IndexFunc(h.teams, func(t github.Team) bool {
			return t.Name == event.Team.Name
		})
		if index == -1 {
			return nil, nil
		}

		team := h.teams[index]

		payload, err := handleTeamEvent(log, h.slack.TeamTmpl(), &team, event)
		h.teams[index] = team

		return payload, err
	} else if event.Workflow != nil {
		var timestamp string
		id := event.Workflow.HeadSHA
		timestamp, err := h.redis.Get(ctx, id).Result()
		if err != nil && err != redis.Nil {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		if timestamp != "" {
			if err := h.slack.PostWorkflowReaction(log, team, event, timestamp); err != nil {
				log.Error("error posting workflow reaction", "err", err.Error())
			}
		}

		return handleWorkflowEvent(log, h.slack.WorkflowTmpl(), team, event)
	} else {
		log.Info("unknown event type")
	}

	return nil, nil
}

func handleCommitEvent(log *slog.Logger, tmpl template.Template, team github.Team, event github.Event, githubClient github.Client) ([]byte, error) {
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
	return slack.CreateCommitMessage(tmpl, team.SlackChannels.Commits, event, team, githubClient)
}

func handleIssueEvent(log *slog.Logger, tmpl template.Template, team github.Team, threadTimestamp string, event github.Event) ([]byte, error) {
	if !slices.Contains([]string{"opened", "closed"}, event.Action) {
		return nil, nil
	}

	channel := team.SlackChannels.Issues
	if team.Config.ExternalContributorsChannel != "" && !team.IsMember(event.User.Login) {
		channel = team.Config.ExternalContributorsChannel
	}

	if channel == "" {
		return nil, nil
	}

	log.Info("Received issue", "slack_channel", channel)
	return slack.CreateIssueMessage(tmpl, channel, threadTimestamp, event)
}

func handlePullRequestEvent(log *slog.Logger, tmpl template.Template, team github.Team, threadTimestamp string, event github.Event) ([]byte, error) {
	if !slices.Contains([]string{"opened", "closed", "reopened"}, event.Action) {
		return nil, nil
	}

	channel := team.SlackChannels.PullRequests
	if team.Config.ExternalContributorsChannel != "" && !team.IsMember(event.User.Login) {
		channel = team.Config.ExternalContributorsChannel
	}

	if channel == "" {
		return nil, nil
	}

	log.Info("Received pull request", "slack_channel", channel)
	return slack.CreatePullRequestMessage(tmpl, channel, threadTimestamp, event)
}

func handleRemoveRepositoryEvent(log *slog.Logger, tmpl template.Template, team *github.Team, event github.Event) ([]byte, error) {
	log.Info("Received repository removed")

	for _, repository := range event.RepositoriesRemoved {
		team.RemoveRepository(repository.Name)
	}

	if team.SlackChannels.Commits == "" {
		return nil, nil
	}

	return slack.CreateTeamMessage(tmpl, team.SlackChannels.Commits, event)
}

func handleRenamedRepository(log *slog.Logger, tmpl template.Template, team *github.Team, event github.Event) ([]byte, error) {
	log.Info("Received repository renamed")

	team.AddRepository(event.Repository.Name)
	team.RemoveRepository(event.Changes.Repository.Name.From)

	if team.SlackChannels.Commits == "" {
		return nil, nil
	}

	return slack.CreateTeamMessage(tmpl, team.SlackChannels.Commits, event)
}

func handleTeamEvent(log *slog.Logger, tmpl template.Template, team *github.Team, event github.Event) ([]byte, error) {
	if event.Action != "added_to_repository" && event.Action != "removed_from_repository" {
		return nil, nil
	}

	log.Info("Received team event")
	if event.Action == "added_to_repository" {
		team.AddRepository(event.Repository.Name)
	} else {
		team.RemoveRepository(event.Repository.Name)
	}

	if team.SlackChannels.Commits == "" {
		return nil, nil
	}

	return slack.CreateTeamMessage(tmpl, team.SlackChannels.Commits, event)
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
