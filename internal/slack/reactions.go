package slack

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/navikt/ghep/internal/github"
)

func (c Client) PostWorkflowReaction(log *slog.Logger, team github.Team, event github.Event, timestamp string) error {
	reaction := "dogcited"
	if event.Action == "requested" && event.Workflow.Status == "queued" {
		reaction = "eyes"
	}

	if event.Action == "in_progress" && event.Workflow.Status == "in_progress" {
		reaction = "hourglass_with_flowing_sand"
	}

	if event.Action == "completed" && event.Workflow.Conclusion == "success" {
		reaction = "white_check_mark"
	}

	if event.Action == "completed" && event.Workflow.Conclusion == "failure" {
		reaction = "x"
	}

	log.Info("Posting reaction to workflow event", "reaction", reaction, "channel", team.SlackChannels.Commits)
	if err := c.reactionRequest("add", reaction, team.SlackChannels.Commits, timestamp); err != nil {
		return fmt.Errorf("posting reaction to workflow event: %v", err)
	}

	return c.RemoveOtherReactions(log, team.SlackChannels.Commits, timestamp, reaction)
}

func (c Client) RemoveOtherReactions(log *slog.Logger, channel, timestamp, reaction string) error {
	log.Info("Removing other reactions", "channel", channel, "timestamp", timestamp, "current_reaction", reaction)
	reactions, err := c.GetReactions(channel, timestamp)
	if err != nil {
		return fmt.Errorf("getting reactions: %v", err)
	}

	for _, r := range reactions {
		if r != reaction {
			log.Info("Removing reaction", "reaction", r)
			if err := c.reactionRequest("remove", r, channel, timestamp); err != nil {
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
	_, err = c.request("reactions.get", marshalled)
	return nil, fmt.Errorf("reactions.get: %v", err)
}

func (c Client) reactionRequest(method, channel, reaction, timestamp string) error {
	payload := map[string]string{
		"channel":   channel,
		"name":      reaction,
		"timestamp": timestamp,
	}

	marshalled, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = c.request("reactions."+method, marshalled)
	return err
}