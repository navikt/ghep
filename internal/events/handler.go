package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strconv"
	"strings"
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
	if team.Config.SilenceDepedabot() && event.Sender.IsDependabot() {
		return nil
	}

	if slices.Contains(team.Config.IgnoreRepositories, event.FindRepositoryName()) {
		return nil
	}

	message, err := h.handle(ctx, log, team, event)
	if err != nil {
		return err
	}

	if message == nil {
		return nil
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	ts, err := h.slack.PostMessage(payload)
	if err != nil {
		return err
	}

	if err := h.redis.Set(ctx, ts, string(payload), oneYear).Err(); err != nil {
		log.Error("error setting message", "err", err.Error(), "ts", ts)
	}

	if id := saveEventSlackResponse(ts, event); id != "" {
		if err := h.redis.Set(ctx, id, ts, oneYear).Err(); err != nil {
			log.Error("error setting thread timestamp", "err", err.Error(), "id", id, "ts", ts)
		}
	}

	return nil
}

func saveEventSlackResponse(ts string, event github.Event) string {
	if ts == "" {
		return ""
	} else if event.Issue != nil && event.Action == "opened" {
		return strconv.Itoa(event.Issue.ID)
	} else if event.PullRequest != nil && event.Action == "opened" {
		return strconv.Itoa(event.PullRequest.ID)
	} else if strings.HasPrefix(event.Ref, refHeadsPrefix) {
		return event.After
	}

	return ""
}

func (h Handler) handle(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	if strings.HasPrefix(event.Ref, refHeadsPrefix) {
		return handleCommitEvent(log, team, event, h.github)
	} else if event.Issue != nil {
		id := strconv.Itoa(event.Issue.ID)
		threadTimestamp, err := h.redis.Get(ctx, id).Result()
		if err != nil && err != redis.Nil {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		return handleIssueEvent(log, team, threadTimestamp, event)
	} else if event.PullRequest != nil {
		id := strconv.Itoa(event.PullRequest.ID)
		threadTimestamp, err := h.redis.Get(ctx, id).Result()
		if err != nil && err != redis.Nil {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		return handlePullRequestEvent(log, team, threadTimestamp, event)
	} else if event.Action == "removed" {
		return handleRemoveRepositoryEvent(log, &team, event)
	} else if event.Action == "renamed" {
		return handleRenamedRepository(log, &team, event)
	} else if event.Team != nil {
		index := slices.IndexFunc(h.teams, func(t github.Team) bool {
			return t.Name == event.Team.Name
		})
		if index == -1 {
			return nil, nil
		}

		team := h.teams[index]

		payload, err := handleTeamEvent(log, &team, event)
		h.teams[index] = team

		return payload, err
	} else if event.Workflow != nil {
		id := event.Workflow.HeadSHA
		timestamp, err := h.redis.Get(ctx, id).Result()
		if err != nil && err != redis.Nil {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		if timestamp != "" && team.SlackChannels.Commits != "" {
			if err := h.slack.PostWorkflowReaction(log, event, team.SlackChannels.Commits, timestamp); err != nil {
				log.Error("error posting workflow reaction", "err", err.Error())
			}

			msg, err := h.redis.Get(ctx, timestamp).Result()
			if err != nil && err != redis.Nil {
				log.Error("error getting message", "err", err.Error(), "timestamp", timestamp)
			}

			if err != redis.Nil {
				if err := h.slack.PostUpdatedCommitMessage(msg, event, timestamp); err != nil {
					log.Error("error updating message", "err", err.Error(), "timestamp", timestamp)
				}
			}
		}

		if err := event.Workflow.UpdateFailedJob(); err != nil {
			log.Error("error updating failed job", "err", err.Error())
		}

		return handleWorkflowEvent(log, team, event)
	} else {
		log.Info("unknown event type")
	}

	return nil, nil
}

func handleCommitEvent(log *slog.Logger, team github.Team, event github.Event, githubClient github.Client) (*slack.Message, error) {
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
	return slack.CreateCommitMessage(log, team.SlackChannels.Commits, event, team, githubClient)
}

func handleIssueEvent(log *slog.Logger, team github.Team, threadTimestamp string, event github.Event) (*slack.Message, error) {
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
	return slack.CreateIssueMessage(channel, threadTimestamp, event), nil
}

func handlePullRequestEvent(log *slog.Logger, team github.Team, threadTimestamp string, event github.Event) (*slack.Message, error) {
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
	return slack.CreatePullRequestMessage(channel, threadTimestamp, event), nil
}

func handleRemoveRepositoryEvent(log *slog.Logger, team *github.Team, event github.Event) (*slack.Message, error) {
	log.Info("Received repository removed")

	for _, repository := range event.RepositoriesRemoved {
		team.RemoveRepository(repository.Name)
	}

	if team.SlackChannels.Commits == "" {
		return nil, nil
	}

	return slack.CreateRemovedMessage(team.SlackChannels.Commits, event), nil
}

func handleRenamedRepository(log *slog.Logger, team *github.Team, event github.Event) (*slack.Message, error) {
	log.Info("Received repository renamed")

	team.AddRepository(event.Repository.Name)
	team.RemoveRepository(event.Changes.Repository.Name.From)

	if team.SlackChannels.Commits == "" {
		return nil, nil
	}

	return slack.CreateRenamedMessage(team.SlackChannels.Commits, event), nil
}

func handleTeamEvent(log *slog.Logger, team *github.Team, event github.Event) (*slack.Message, error) {
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

	return slack.CreateTeamMessage(team.SlackChannels.Commits, event), nil
}

func handleWorkflowEvent(log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
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
	return slack.CreateWorkflowMessage(team.SlackChannels.Workflows, event), nil
}
