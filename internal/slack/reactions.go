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

	return nil
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
	return fmt.Errorf("reactions.%s: %v", method, err)
}
