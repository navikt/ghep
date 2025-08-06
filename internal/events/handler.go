package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql/gensql"
)

type Handler struct {
	github      github.Client
	db          *gensql.Queries
	slack       slack.Client
	teamsConfig map[string]github.Team
}

func NewHandler(githubClient github.Client, db *gensql.Queries, slackClient slack.Client, teamsConfig map[string]github.Team) Handler {
	return Handler{
		github:      githubClient,
		db:          db,
		slack:       slackClient,
		teamsConfig: teamsConfig,
	}
}

func eventIsFromDependabot(event github.Event) bool {
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

	return false
}

func (h *Handler) Handle(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) error {
	if team.Config.ShouldSilenceDependabot() && eventIsFromDependabot(event) {
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
		log.Error("error posting message", "err", err.Error(), "channel", message.Channel, "timestamp", message.ThreadTimestamp)
		return err
	}

	if err := h.storeEvent(ctx, log, event, team, resp, payload); err != nil {
		log.Error("error storing event", "err", err.Error(), "event_id", getEventID(event), "team", team.Name)
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

		for name, t := range h.teamsConfig {
			if name == team.Name {
				t.SlackChannels = channels
				t.Config.ExternalContributorsChannel = team.Config.ExternalContributorsChannel
				h.teamsConfig[t.Name] = t
				break
			}
		}

		if err := h.slack.JoinChannel(resp.Channel); err != nil {
			log.Error("error joining channel", "err", err.Error(), "channel", message.Channel, "channel_id", resp.Channel)
		}
	}

	return nil
}

// getEventID returns the event ID based on the type of event.
// Some events are not supported, so we return an empty string for those.
func getEventID(event github.Event) string {
	if event.IsCommit() {
		return event.After
	} else if event.Issue != nil && event.Action == "opened" {
		return strconv.Itoa(event.Issue.ID)
	} else if event.PullRequest != nil && event.Action == "opened" {
		return strconv.Itoa(event.PullRequest.ID)
	} else if event.Alert != nil && event.Action == "created" {
		return event.Alert.URL
	} else if event.Workflow != nil && event.Action == "completed" && event.Workflow.Conclusion == "failure" {
		return strconv.Itoa(event.Workflow.ID)
	} else if event.Release != nil && event.Action == "published" {
		return strconv.Itoa(event.Release.ID)
	}

	return ""
}

func (h *Handler) storeEvent(ctx context.Context, log *slog.Logger, event github.Event, team github.Team, resp *slack.MessageResponse, payload []byte) error {
	id := getEventID(event)
	if id == "" {
		return nil
	}

	if err := h.db.CreateSlackMessage(ctx, gensql.CreateSlackMessageParams{
		TeamSlug: team.Name,
		EventID:  id,
		ThreadTs: resp.Timestamp,
		Channel:  resp.Channel,
		Payload:  payload,
	}); err != nil {
		log.Error("error storing message", "err", err.Error(), "ts", resp.Timestamp)
	}

	return nil
}

func (h *Handler) handle(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	eventType := event.GetEventType()
	log = log.With("event_type", eventType.String())

	switch eventType {
	case github.TypeCommit:
		return handleCommitEvent(ctx, log, team, event, h.db)
	case github.TypeCodeScanningAlert:
		return h.handleCodeScanningAlertEvent(ctx, log, team, event)
	case github.TypeDependabotAlert:
		return h.handleDependabotAlertEvent(ctx, log, team, event)
	case github.TypeIssue:
		return h.handleIssueEvent(ctx, log, team, event)
	case github.TypePullRequest:
		return h.handlePullRequestEvent(ctx, log, team, event)
	case github.TypeRelease:
		return h.handleReleaseEvent(ctx, log, team, event)
	case github.TypeRepositoryRenamed:
		return h.handleRenamedRepository(ctx, log, team, event)
	case github.TypeRepositoryPublic:
		return handlePublicRepositoryEvent(log, team, event)
	case github.TypeSecurityAdvisory:
		return h.handleSecurityAdvisoryEvent(ctx, log, team, event)
	case github.TypeSecretScanningAlert:
		return h.handleSecretScanningAlertEvent(ctx, log, team, event)
	case github.TypeTeam:
		return h.handleTeamEvent(ctx, log, event)
	case github.TypeWorkflow:
		return h.handleWorkflowEvent(ctx, log, team, event)
	}

	return nil, nil
}

func handleCommitEvent(ctx context.Context, log *slog.Logger, team github.Team, event github.Event, db *gensql.Queries) (*slack.Message, error) {
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
	return slack.CreateCommitMessage(ctx, log, db, team.SlackChannels.Commits, event)
}

func (h *Handler) handleRenamedRepository(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	log.Info("Received repository renamed")

	h.db.UpdateRepository(ctx, gensql.UpdateRepositoryParams{
		Name:    event.Repository.Name,
		OldName: event.Changes.Repository.Name.From,
	})

	if team.SlackChannels.Commits == "" {
		return nil, nil
	}

	log.Info("Posting renamed repository message", "slack_channel", team.SlackChannels.Commits)
	return slack.CreateRenamedMessage(team.SlackChannels.Commits, event), nil
}

func handlePublicRepositoryEvent(log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	if team.SlackChannels.Commits == "" {
		return nil, nil
	}

	log.Info("Received repository publicized", "slack_channel", team.SlackChannels.Commits)
	return slack.CreatePublicizedMessage(team.SlackChannels.Commits, event), nil
}
