package events

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/redis/go-redis/v9"
)

func (h *Handler) handleWorkflowEvent(ctx context.Context, log *slog.Logger, team github.Team, event github.Event) (*slack.Message, error) {
	gitCommitSHA := event.Workflow.HeadSHA
	commitTimestamp, err := h.redis.Get(ctx, gitCommitSHA).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Error("error getting thread timestamp", "err", err.Error(), "id", gitCommitSHA)
	}

	if commitTimestamp != "" {
		if team.SlackChannels.Commits != "" {
			if !event.Sender.IsBot() {
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
	}

	workflowID := strconv.Itoa(event.Workflow.ID)
	workflowTimestamp, err := h.redis.Get(ctx, workflowID).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Error("error getting workflow timestamp", "err", err.Error(), "id", workflowID)
	}

	if workflowTimestamp != "" {
		if event.Action == "completed" && event.Workflow.Conclusion == "success" {
			log.Info("Reacting to workflow", "action", event.Action, "workflow_status", event.Workflow.Status, "workflow_conclusion", event.Workflow.Conclusion)
			if err := h.slack.PostReaction(team.SlackChannels.Workflows, workflowTimestamp, slack.ReactionSuccess); err != nil {
				log.Error("error posting reaction", "err", err.Error(), "channel", team.SlackChannels.Workflows, "timestamp", workflowTimestamp)
			}
		}
	}

	if err := event.Workflow.UpdateFailedJob(); err != nil {
		log.Error("error updating failed job", "err", err.Error())
	}

	return handleWorkflowEvent(log, team, event)
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

	if len(team.Config.Workflows.Repositories) > 0 && !slices.Contains(team.Config.Workflows.Repositories, event.Repository.Name) {
		return nil, nil
	}

	if len(team.Config.Workflows.Workflows) > 0 && !slices.Contains(team.Config.Workflows.Workflows, event.Workflow.Name) {
		return nil, nil
	}

	if event.Action != "completed" || event.Workflow.Conclusion != "failure" {
		return nil, nil
	}

	log.Info("Received workflow run", "conclusion", event.Workflow.Conclusion, "slack_channel", team.SlackChannels.Workflows)
	return slack.CreateWorkflowMessage(team.SlackChannels.Workflows, event), nil
}
