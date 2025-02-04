package slack

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/navikt/ghep/internal/github"
)

const (
	ReactionSuccess    = "white_check_mark"
	ReactionFailure    = "x"
	ReactionCancelled  = "parking"
	ReactionQueued     = "eyes"
	ReactionInProgress = "hourglass_flowing_sand"
	ReactionDefault    = "dogcited"
)

func (c Client) PostWorkflowReaction(log *slog.Logger, event github.Event, channel, timestamp string) error {
	reaction := ReactionDefault
	if event.Action == "requested" && event.Workflow.Status == "queued" {
		reaction = ReactionQueued
	} else if event.Action == "in_progress" && event.Workflow.Status == "in_progress" {
		reaction = ReactionInProgress
	} else if event.Action == "completed" {
		switch event.Workflow.Conclusion {
		case "success":
			reaction = ReactionSuccess
		case "failure":
			reaction = ReactionFailure
		case "cancelled":
			reaction = ReactionCancelled
		}

		log = log.With("conclusion", event.Workflow.Conclusion)
	}

	if reaction == "dogcited" {
		log.Info("No reaction found for event (still reacting)", "action", event.Action, "status", event.Workflow.Status, "conclusion", event.Workflow.Conclusion, "event", event)
	}

	log.Info("Posting reaction to workflow event", "reaction", reaction, "channel", channel, "timestamp", timestamp)
	if err := c.reactionRequest("add", channel, timestamp, reaction); err != nil {
		return fmt.Errorf("posting reaction to workflow event: %v", err)
	}

	return c.RemoveOtherReactions(log, channel, timestamp, reaction)
}

func (c Client) RemoveOtherReactions(log *slog.Logger, channel, timestamp, currentReaction string) error {
	reactions, err := c.GetReactions(channel, timestamp)
	if err != nil {
		return fmt.Errorf("getting reactions: %v", err)
	}

	for _, reaction := range reactions {
		if reaction != currentReaction {
			log.Info("Removing reaction", "reaction", reaction, "channel", channel, "timestamp", timestamp, "current_reaction", currentReaction)
			if err := c.reactionRequest("remove", channel, timestamp, reaction); err != nil {
				return fmt.Errorf("removing reaction: %v", err)
			}
		}
	}

	return nil
}

func (c Client) GetReactions(channel, timestamp string) ([]string, error) {
	resp, err := c.getRequest("reactions.get", channel, timestamp)
	if err != nil {
		c.log.Error("Error getting reactions", "response", resp, "error", err)
		return nil, err
	}

	var reactions []string
	for _, reaction := range resp.Message.Reactions {
		reactions = append(reactions, reaction.Name)
	}

	return reactions, nil
}

func (c Client) reactionRequest(method, channel, timestamp, reaction string) error {
	payload := map[string]string{
		"channel":   channel,
		"name":      reaction,
		"timestamp": timestamp,
	}

	marshalled, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := c.postRequest("reactions."+method, marshalled)
	if err != nil {
		if resp != nil && (resp.Error == "already_reacted" || resp.Error == "no_reaction") {
			return nil
		}

		return err
	}

	return nil
}
