package slack

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/navikt/ghep/internal/github"
)

func (c Client) PostWorkflowReaction(log *slog.Logger, event github.Event, channel, timestamp string) error {
	reaction := "dogcited"
	if event.Action == "requested" && event.Workflow.Status == "queued" {
		reaction = "eyes"
	}

	if event.Action == "in_progress" && event.Workflow.Status == "in_progress" {
		reaction = "hourglass_flowing_sand"
	}

	if event.Action == "completed" && event.Workflow.Conclusion == "success" {
		reaction = "white_check_mark"
	}

	if event.Action == "completed" && event.Workflow.Conclusion == "failure" {
		reaction = "x"
	}

	log.Info("Posting reaction to workflow event", "reaction", reaction, "channel", channel, "timestamp", timestamp)
	if err := c.reactionRequest("add", channel, timestamp, reaction); err != nil {
		return fmt.Errorf("posting reaction to workflow event: %v", err)
	}

	return c.RemoveOtherReactions(log, channel, timestamp, reaction)
}

func (c Client) RemoveOtherReactions(log *slog.Logger, channel, timestamp, current_reaction string) error {
	log.Info("Removing other reactions", "channel", channel, "timestamp", timestamp, "current_reaction", current_reaction)
	reactions, err := c.GetReactions(channel, timestamp)
	if err != nil {
		return fmt.Errorf("getting reactions: %v", err)
	}

	for _, reaction := range reactions {
		if reaction != current_reaction {
			log.Info("Removing reaction", "reaction", reaction)
			if err := c.reactionRequest("remove", channel, timestamp, reaction); err != nil {
				return fmt.Errorf("removing reaction: %v", err)
			}
		}
	}

	return nil
}

func (c Client) GetReactions(channel, timestamp string) ([]string, error) {
	payload := map[string]string{
		"channel":   channel,
		"timestamp": timestamp,
	}

	marshalled, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.request("reactions.get", marshalled)
	if err != nil {
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
	if err != nil && resp != nil && resp.Error == "already_reacted" {
		return nil
	}

	return err
}
