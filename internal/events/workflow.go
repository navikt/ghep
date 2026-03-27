package events

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql/gensql"
)

func (h *Handler) handleWorkflowEvent(ctx context.Context, log *slog.Logger, team github.Team, source github.Source, event github.Event) (*slack.Message, error) {
	gitCommitSHA := event.Workflow.HeadSHA

	// React to commit messages across all channels that have the commit
	commitMessages, err := h.db.ListSlackMessagesByEvent(ctx, gensql.ListSlackMessagesByEventParams{
		TeamSlug: team.Name,
		EventID:  gitCommitSHA,
	})
	if err != nil {
		log.Error("Listing commit messages", "error", err, "id", gitCommitSHA)
	}

	for _, commitMessage := range commitMessages {
		if !event.Sender.IsBot() {
			if err := h.slack.PostWorkflowReaction(log, event, commitMessage.Channel, commitMessage.ThreadTs); err != nil {
				log.Error("Posting workflow reaction", "error", err, "channel", commitMessage.Channel, "timestamp", commitMessage.ThreadTs)
			}

			if commitMessage.Payload != nil {
				updatedCommitMessage, err := slack.CreateUpdatedCommitMessage(commitMessage.Payload, event)
				if err != nil {
					log.Error("Updating message", "error", err, "timestamp", commitMessage.ThreadTs)
					continue
				}
				updatedCommitMessage.Timestamp = commitMessage.ThreadTs

				log.Info("Posting update of commit", "channel", updatedCommitMessage.Channel, "timestamp", updatedCommitMessage.Timestamp)
				if err = h.slack.PostUpdatedMessage(*updatedCommitMessage); err != nil {
					log.Error("Posting updated commit message", "error", err)
				}
			}
		}
	}

	workflowID := strconv.Itoa(event.Workflow.ID)
	workflowMessage, err := h.db.GetSlackMessage(ctx, gensql.GetSlackMessageParams{
		TeamSlug: team.Name,
		EventID:  workflowID,
		Channel:  source.Channel,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Error("Getting workflow timestamp", "error", err, "id", workflowID)
	}

	if workflowMessage.ThreadTs != "" {
		workflowTimestamp := workflowMessage.ThreadTs
		if event.Action == "completed" && event.Workflow.Conclusion == "success" {
			log.Info("Reacting to workflow", "action", event.Action, "workflow_status", event.Workflow.Status, "workflow_conclusion", event.Workflow.Conclusion)
			if err := h.slack.PostReaction(source.Channel, workflowTimestamp, slack.ReactionSuccess); err != nil {
				log.Error("Posting reaction", "error", err, "channel", source.Channel, "timestamp", workflowTimestamp)
			}
		}
	}

	if err := event.Workflow.UpdateFailedJob(); err != nil {
		log.Error("Updating failed job", "error", err)
	}

	return handleWorkflowEvent(log, source, event)
}

func handleWorkflowEvent(log *slog.Logger, source github.Source, event github.Event) (*slack.Message, error) {
	if source.Config.Workflows.IgnoreBots && event.Sender.IsBot() {
		return nil, nil
	}

	if len(source.Config.Workflows.Repositories) > 0 && !slices.Contains(source.Config.Workflows.Repositories, event.Repository.Name) {
		return nil, nil
	}

	if len(source.Config.Workflows.Workflows) > 0 && !slices.Contains(source.Config.Workflows.Workflows, event.Workflow.Name) {
		return nil, nil
	}

	if event.Action != "completed" || event.Workflow.Conclusion != "failure" {
		return nil, nil
	}

	log.Info("Received workflow run", "conclusion", event.Workflow.Conclusion, "channel", source.Channel)
	return slack.CreateWorkflowMessage(source.Channel, event), nil
}
