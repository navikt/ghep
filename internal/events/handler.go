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
	oneYear = 8760 * time.Hour
)

type Handler struct {
	github github.Client
	redis  *redis.Client
	slack  slack.Client
	teams  []*github.Team
}

func NewHandler(githubClient github.Client, redis *redis.Client, slackClient slack.Client, teams []*github.Team) Handler {
	return Handler{
		github: githubClient,
		redis:  redis,
		slack:  slackClient,
		teams:  teams,
	}
}

func shouldSilenceBots(team github.Team, event github.Event) bool {
	if team.Config.ShouldSilenceDependabot() {
		if event.Sender.IsBot() {
			// We don't want to ignore pull request merges from Atlantis
			if event.PullRequest != nil && event.PullRequest.Action == "closed" && strings.Contains(event.Sender.Login, "atlantis") {
				return false
			}

			return true
		}

		// Teams use different bots for merging pull requests, so we need to check the author of the pull request
		if event.PullRequest != nil && event.PullRequest.User.IsBot() {
			return true
		}

		if event.IsCommit() {
			for _, commit := range event.Commits {
				if commit.Author.IsBot() {
					return true
				}
			}
		}
	}

	return false
}

func (h *Handler) Handle(ctx context.Context, log *slog.Logger, team *github.Team, event github.Event) error {
	if shouldSilenceBots(*team, event) {
		return nil
	}

	if slices.Contains(team.Config.IgnoreRepositories, event.GetRepositoryName()) {
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
		if team.Config.ExternalContributorsChannel == message.Channel {
			team.Config.ExternalContributorsChannel = resp.Channel
		}

		for i, t := range h.teams {
			if t.Name == team.Name {
				h.teams[i].SlackChannels = channels
				h.teams[i].Config.ExternalContributorsChannel = team.Config.ExternalContributorsChannel
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
	} else if event.IsCommit() {
		return event.After
	} else if event.Issue != nil && event.Action == "opened" {
		return strconv.Itoa(event.Issue.ID)
	} else if event.PullRequest != nil && event.Action == "opened" {
		return strconv.Itoa(event.PullRequest.ID)
	} else if event.Alert != nil && event.Action == "created" {
		return event.Alert.URL
	} else if event.Workflow != nil && event.Action == "completed" && event.Workflow.Conclusion == "failure" {
		return strconv.Itoa(event.Workflow.ID)
	}

	return ""
}

func (h *Handler) handle(ctx context.Context, log *slog.Logger, team *github.Team, event github.Event) (*slack.Message, error) {
	eventType := event.GetEventType()
	log = log.With("event_type", eventType)

	switch eventType {
	case github.TypeCommit:
		return handleCommitEvent(log, *team, event, h.github)
	case github.TypeCodeScanningAlert:
		return h.handleCodeScanningAlertEvent(ctx, log, *team, event)
	case github.TypeDependabotAlert:
		return h.handleDependabotAlertEvent(ctx, log, *team, event)
	case github.TypeIssue:
		return h.handleIssueEvent(ctx, log, *team, event)
	case github.TypePullRequest:
		return h.handlePullRequestEvent(ctx, log, *team, event)
	case github.TypeRelease:
		return h.handleReleaseEvent(ctx, log, *team, event)
	case github.TypeRepositoryRenamed:
		return handleRenamedRepository(log, team, event)
	case github.TypeRepositoryPublic:
		return handlePublicRepositoryEvent(log, team, event)
	case github.TypeSecurityAdvisory:
		return h.handleSecurityAdvisoryEvent(ctx, log, *team, event)
	case github.TypeSecretScanningAlert:
		return h.handleSecretScanningAlertEvent(ctx, log, *team, event)
	case github.TypeTeam:
		return h.handleTeamEvent(ctx, log, event)
	case github.TypeWorkflow:
		return h.handleWorkflowEvent(ctx, log, *team, event)
	}

	return nil, nil
}

func handleCommitEvent(log *slog.Logger, team github.Team, event github.Event, githubClient github.Client) (*slack.Message, error) {
	if team.SlackChannels.Commits == "" {
		return nil, nil
	}

	branch := strings.TrimPrefix(event.Ref, github.RefHeadsPrefix)
	if branch != event.Repository.DefaultBranch {
		return nil, nil
	}

	if len(event.Commits) == 0 {
		return nil, nil
	}

	log = log.With("slack_channel", team.SlackChannels.Commits)
	log.Info("Received commit event")
	return slack.CreateCommitMessage(log, team.SlackChannels.Commits, event, team, githubClient)
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

	log = log.With("slack_channel", team.SlackChannels.Commits)
	log.Info("Received repository publicized")
	return slack.CreatePublicizedMessage(team.SlackChannels.Commits, event), nil
}
