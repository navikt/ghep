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
	if event.IsCodeQLWorkflow() {
		return nil
	}

	if team.Config.ShouldSilenceDependabot() && eventIsFromDependabot(event) {
		return nil
	}

	if slices.Contains(team.Config.IgnoreRepositories, event.GetRepositoryName()) {
		return nil
	}

	eventType := event.GetEventType()
	log = log.With("event_type", eventType.String())

	// Handle one-time DB side effects before iterating over sources
	switch eventType {
	case github.TypeRepositoryRenamed:
		if err := h.db.UpdateRepository(ctx, gensql.UpdateRepositoryParams{
			Name:    event.Repository.Name,
			OldName: event.Changes.Repository.Name.From,
		}); err != nil {
			return err
		}
	case github.TypeTeam:
		if err := h.handleTeamSideEffects(ctx, log, event); err != nil {
			return err
		}
	}

	sources := team.SourcesForType(eventType)
	for _, source := range sources {
		if err := h.handleSource(ctx, log, team, source, event, eventType); err != nil {
			log.Error("Handling source", "error", err, "source_type", source.SourceType, "channel", source.Channel)
		}
	}

	return nil
}

func (h *Handler) handleSource(ctx context.Context, log *slog.Logger, team github.Team, source github.Source, event github.Event, eventType github.EventType) error {
	if source.Channel == "" {
		return nil
	}

	log = log.With("channel", source.Channel)

	message, err := h.handleForSource(ctx, log, team, source, event, eventType)
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
		log.Error("Posting message", "error", err, "channel", message.Channel, "timestamp", message.ThreadTimestamp)
		return err
	}

	if err := h.storeEvent(ctx, log, event, team, resp, payload); err != nil {
		log.Error("Storing event", "error", err, "event_id", getEventID(event), "team", team.Name)
	}

	// Update source channel name to ID if Slack returned a different channel identifier
	if message.Channel != resp.Channel {
		h.updateSourceChannelID(team, source, message.Channel, resp.Channel)

		if err := h.slack.JoinChannel(resp.Channel); err != nil {
			log.Error("Joining channel", "error", err, "channel", message.Channel, "channel_id", resp.Channel)
		}
	}

	return nil
}

func (h *Handler) handleForSource(ctx context.Context, log *slog.Logger, team github.Team, source github.Source, event github.Event, eventType github.EventType) (*slack.Message, error) {
	if len(source.Config.Branches) > 0 {
		branch := eventBranch(event, eventType)
		if branch != "" && !slices.Contains(source.Config.Branches, branch) {
			return nil, nil
		}
	}

	switch eventType {
	case github.TypeCommit:
		return handleCommitEvent(ctx, log, source, event, h.db)
	case github.TypeCodeScanningAlert:
		return h.handleCodeScanningAlertEvent(ctx, log, team, source, event)
	case github.TypeDependabotAlert:
		return h.handleDependabotAlertEvent(ctx, log, team, source, event)
	case github.TypeIssue:
		return h.handleIssueEvent(ctx, log, team, source, event)
	case github.TypePullRequest:
		return h.handlePullRequestEvent(ctx, log, team, source, event)
	case github.TypeRelease:
		return h.handleReleaseEvent(ctx, log, team, source, event)
	case github.TypeRepositoryRenamed:
		log.Info("Posting renamed repository message", "channel", source.Channel)
		return slack.CreateRenamedMessage(source.Channel, event), nil
	case github.TypeRepositoryPublic:
		log.Info("Received repository publicized", "channel", source.Channel)
		return slack.CreatePublicizedMessage(source.Channel, event), nil
	case github.TypeSecurityAdvisory:
		return h.handleSecurityAdvisoryEvent(ctx, log, team, source, event)
	case github.TypeSecretScanningAlert:
		return h.handleSecretScanningAlertEvent(ctx, log, team, source, event)
	case github.TypeTeam:
		return handleTeamEvent(log, source.Channel, event)
	case github.TypeWorkflow:
		return h.handleWorkflowEvent(ctx, log, team, source, event)
	}

	return nil, nil
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
		log.Error("Storing message", "error", err, "timestamp", resp.Timestamp)
	}

	return nil
}

// updateSourceChannelID updates the source channel from name to Slack channel ID in the teamsConfig.
func (h *Handler) updateSourceChannelID(team github.Team, source github.Source, oldChannel, newChannel string) {
	for name, t := range h.teamsConfig {
		if name != team.Name {
			continue
		}

		for i := range t.Sources {
			if t.Sources[i].Channel == oldChannel {
				t.Sources[i].Channel = newChannel
			}
		}

		if t.Config.ExternalContributorsChannel == oldChannel {
			t.Config.ExternalContributorsChannel = newChannel
		}

		h.teamsConfig[name] = t
		break
	}
}

// eventBranch returns the branch associated with an event for a given event type.
// Returns an empty string for event types that have no branch context.
func eventBranch(event github.Event, eventType github.EventType) string {
	switch eventType {
	case github.TypeCommit:
		return strings.TrimPrefix(event.Ref, github.RefHeadsPrefix)
	case github.TypeWorkflow:
		if event.Workflow != nil {
			return event.Workflow.HeadBranch
		}
	case github.TypePullRequest:
		if event.PullRequest != nil {
			return event.PullRequest.Base.Ref
		}
	}
	return ""
}

func handleCommitEvent(ctx context.Context, log *slog.Logger, source github.Source, event github.Event, db *gensql.Queries) (*slack.Message, error) {
	branch := strings.TrimPrefix(event.Ref, github.RefHeadsPrefix)

	if len(source.Config.Branches) == 0 && branch != event.Repository.DefaultBranch {
		return nil, nil
	}

	if len(event.Commits) == 0 {
		return nil, nil
	}

	log = log.With("channel", source.Channel)
	log.Info("Received commit event")
	return slack.CreateCommitMessage(ctx, log, db, source.Channel, event)
}
