package events

import (
	"context"
	"encoding/json"
	"errors"
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
	oneYear = 8760 * time.Hour
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

func shouldSilenceBots(team github.Team, event github.Event) bool {
	if team.Config.ShouldSilenceDependabot() {
		if event.Sender.IsDependabot() {
			return true
		}

		// Teams use different bots for merging pull requests, so we need to check the author of the pull request
		if event.PullRequest != nil && event.PullRequest.User.IsDependabot() {
			return true
		}

		if event.IsCommit() {
			for _, commit := range event.Commits {
				if commit.Author.IsDependabot() {
					return true
				}
			}
		}
	}

	return false
}

func (h *Handler) Handle(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) error {
	if shouldSilenceBots(team, event) {
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

	resp, err := h.slack.PostMessage(payload)
	if err != nil {
		log.Error("error posting message", "err", err.Error(), "channel", message.Channel, "timestamp", message.Timestamp)
		return err
	}
	ts := resp.Timestamp

	if err := h.redis.Set(ctx, ts, string(payload), oneYear).Err(); err != nil {
		log.Error("error setting message", "err", err.Error(), "ts", ts)
	}

	if id := saveEventSlackResponse(ts, event); id != "" {
		if err := h.redis.Set(ctx, id, ts, oneYear).Err(); err != nil {
			log.Error("error setting thread timestamp", "err", err.Error(), "id", id, "ts", ts)
		}
	}

	// This checks if we have sent the message with channel name, and not the channel ID
	if message.Channel != resp.Channel {
		channels := team.SlackChannels
		if channels.Commits == message.Channel {
			channels.Commits = resp.Channel
		}
		if channels.Issues == message.Channel {
			channels.Issues = resp.Channel
		}
		if channels.PullRequests == message.Channel {
			channels.PullRequests = resp.Channel
		}
		if channels.Releases == message.Channel {
			channels.Releases = resp.Channel
		}
		if channels.Workflows == message.Channel {
			channels.Workflows = resp.Channel
		}

		for i, t := range h.teams {
			if t.Name == team.Name {
				h.teams[i].SlackChannels = channels
				break
			}
		}

		if err := h.slack.JoinChannel(resp.Channel); err != nil {
			log.Error("error joining channel", "err", err.Error(), "channel", message.Channel, "channel_id", resp.Channel)
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
	} else if event.IsCommit() {
		return event.After
	} else if event.Workflow != nil && event.Action == "completed" && event.Workflow.Conclusion == "failure" {
		return strconv.Itoa(event.Workflow.ID)
	}

	return ""
}

func (h *Handler) handle(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	if event.IsCommit() {
		return handleCommitEvent(log, team, event, h.github)
	} else if event.Issue != nil {
		id := strconv.Itoa(event.Issue.ID)
		timestamp, err := h.redis.Get(ctx, id).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		if !slices.Contains([]string{"opened", "closed", "reopened", "edited"}, event.Action) {
			log.Info("unknown issue action")
			return nil, nil
		}

		msgBytes, err := h.redis.Get(ctx, timestamp).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			log.Error("error getting message", "err", err.Error(), "timestamp", timestamp)
		}

		if !errors.Is(err, redis.Nil) && event.Action != "opened" {
			var oldMessage slack.Message
			if err := json.Unmarshal([]byte(msgBytes), &oldMessage); err != nil {
				return nil, err
			}

			updatedMessage := slack.CreateUpdatedIssueMessage(oldMessage, event)
			updatedMessage.Timestamp = timestamp
			marshalled, err := json.Marshal(updatedMessage)
			if err != nil {
				return nil, err
			}

			log.Info("Posting update of issue", "channel", updatedMessage.Channel, "timestamp", timestamp)
			_, err = h.slack.PostUpdatedMessage(marshalled)
			if err != nil {
				return nil, err
			}

			if slices.Contains([]string{"reopened", "edited"}, event.Action) {
				return nil, nil
			}
		}

		return handleIssueEvent(log, team, timestamp, event)
	} else if event.PullRequest != nil {
		id := strconv.Itoa(event.PullRequest.ID)
		timestamp, err := h.redis.Get(ctx, id).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
		}

		if !slices.Contains([]string{"opened", "closed", "reopened", "edited"}, event.Action) {
			log.Info("unknown pull request action")
			return nil, nil
		}

		messageBytes, err := h.redis.Get(ctx, timestamp).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			log.Error("error getting message", "err", err.Error(), "timestamp", timestamp)
		}

		if !errors.Is(err, redis.Nil) && event.Action != "opened" {
			var oldMessage slack.Message
			if err := json.Unmarshal([]byte(messageBytes), &oldMessage); err != nil {
				return nil, err
			}

			updatedMessage := slack.CreateUpdatedPullRequestMessage(oldMessage, event)
			updatedMessage.Timestamp = timestamp

			marshalled, err := json.Marshal(updatedMessage)
			if err != nil {
				return nil, err
			}

			log.Info("Posting update of pull request", "channel", updatedMessage.Channel, "timestamp", timestamp)
			_, err = h.slack.PostUpdatedMessage(marshalled)
			if err != nil {
				return nil, err
			}

			if slices.Contains([]string{"reopened", "edited"}, event.Action) {
				return nil, nil
			}
		}

		return handlePullRequestEvent(log, team, timestamp, event)
	} else if event.Release != nil {
		var timestamp string
		var err error
		if event.Action == "edited" {
			id := strconv.Itoa(event.Release.ID)
			timestamp, err = h.redis.Get(ctx, id).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				log.Error("error getting thread timestamp", "err", err.Error(), "id", id)
			}
		}

		return handleReleaseEvent(log, team, event, timestamp)
	} else if event.Action == "removed" {
		return handleRemoveRepositoryEvent(log, &team, event)
	} else if event.Action == "renamed" {
		return handleRenamedRepository(log, &team, event)
	} else if event.Action == "publicized" {
		return handlePublicRepositoryEvent(log, &team, event)
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
		gitCommitSHA := event.Workflow.HeadSHA
		commitTimestamp, err := h.redis.Get(ctx, gitCommitSHA).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			log.Error("error getting thread timestamp", "err", err.Error(), "id", gitCommitSHA)
		}

		if commitTimestamp != "" {
			if team.SlackChannels.Commits != "" {
				if err := h.slack.PostWorkflowReaction(log, event, team.SlackChannels.Commits, commitTimestamp); err != nil {
					log.Error("error posting workflow reaction", "err", err.Error(), "channel", team.SlackChannels.Commits, "timestamp", commitTimestamp)
				}

				msg, err := h.redis.Get(ctx, commitTimestamp).Result()
				if err != nil && !errors.Is(err, redis.Nil) {
					log.Error("error getting message", "err", err.Error(), "timestamp", commitTimestamp)
				}

				if !errors.Is(err, redis.Nil) {
					if err := h.slack.PostUpdatedCommitMessage(log, msg, event, commitTimestamp); err != nil {
						log.Error("error updating message", "err", err.Error(), "timestamp", commitTimestamp)
					}
				}
			}
		}

		workflowID := strconv.Itoa(event.Workflow.ID)
		workflowTimestamp, err := h.redis.Get(ctx, workflowID).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			log.Error("error getting workflow timestamp", "err", err.Error(), "id", workflowID)
		}

		if workflowTimestamp != "" {
			log.Info("event", "action", event.Action, "workflow_status", event.Workflow.Status, "workflow_conclusion", event.Workflow.Conclusion)
			if event.Action == "completed" && event.Workflow.Conclusion == "success" {
				if err := h.slack.PostReaction(team.SlackChannels.Workflows, workflowTimestamp, slack.ReactionSuccess); err != nil {
					log.Error("error posting reaction", "err", err.Error(), "channel", team.SlackChannels.Workflows, "timestamp", workflowTimestamp)
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
	branch := strings.TrimPrefix(event.Ref, github.RefHeadsPrefix)

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
	if team.IsExternalContributor(event.Sender) {
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
	if team.IsExternalContributor(event.Sender) {
		channel = team.Config.ExternalContributorsChannel
	}

	if channel == "" {
		return nil, nil
	}

	log.Info("Received pull request", "slack_channel", channel)
	return slack.CreatePullRequestMessage(channel, threadTimestamp, event), nil
}

func handleReleaseEvent(log *slog.Logger, team github.Team, event github.Event, timestamp string) (*slack.Message, error) {
	if !slices.Contains([]string{"published", "edited"}, event.Action) {
		return nil, nil
	}

	if team.SlackChannels.Releases == "" {
		return nil, nil
	}

	log.Info("Received release", "slack_channel", team.SlackChannels.Releases)
	return slack.CreateReleaseMessage(team.SlackChannels.Releases, timestamp, event), nil
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

func handlePublicRepositoryEvent(log *slog.Logger, team *github.Team, event github.Event) (*slack.Message, error) {
	if team.SlackChannels.Commits == "" {
		return nil, nil
	}

	log.Info("Received repository publicized")

	return slack.CreatePublicizedMessage(team.SlackChannels.Commits, event), nil
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

	if team.Config.Workflows.IgnoreBots && event.Sender.IsBot() {
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
