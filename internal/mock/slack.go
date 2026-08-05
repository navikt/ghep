package mock

import (
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

type SlackReactionMetadata struct {
	Channel   string
	Reaction  string
	Timestamp string
}

type Slack struct {
	Messages        []slack.Message
	Reactions       []SlackReactionMetadata
	UpdatedMessages []slack.Message
}

func (s *Slack) Ensure(t *testing.T, eventType github.EventType, messages, reactions, updatedMessages int) {
	s.EnsureMessages(t, eventType, messages)
	s.EnsureReactions(t, eventType, reactions)
	s.EnsureUpdatedMessages(t, eventType, updatedMessages)
}

func (s *Slack) EnsureMessages(t *testing.T, eventType github.EventType, expected int) {
	t.Helper()

	if len(s.Messages) != expected {
		t.Errorf("%s expected %d messages, got %d", eventType, expected, len(s.Messages))
	}
}

func (s *Slack) EnsureReactions(t *testing.T, eventType github.EventType, expected int) {
	t.Helper()

	if len(s.Reactions) != expected {
		t.Errorf("%s expected %d reactions, got %d", eventType, expected, len(s.Reactions))
	}
}

func (s *Slack) EnsureUpdatedMessages(t *testing.T, eventType github.EventType, expected int) {
	t.Helper()

	if len(s.UpdatedMessages) != expected {
		t.Errorf("%s expected %d updated messages, got %d", eventType, expected, len(s.UpdatedMessages))
	}
}

func (s *Slack) EnsureChannels(teams map[string]github.Team) error {
	panic("unimplemented EnsureChannels")
}

func (s *Slack) GetReactions(channel string, timestamp string) ([]string, error) {
	panic("unimplemented GetReactions")
}

func (s *Slack) JoinChannel(channel string) error {
	panic("unimplemented JoinChannel")
}

func (s *Slack) PostMessage(payload []byte) (slack.MessageResponse, error) {
	var message slack.Message
	if err := json.Unmarshal(payload, &message); err != nil {
		return slack.MessageResponse{}, nil
	}

	s.Messages = append(s.Messages, message)
	return slack.MessageResponse{Channel: message.Channel}, nil
}

func (s *Slack) PostReaction(channel string, timestamp string, reaction string) error {
	s.Reactions = append(s.Reactions, SlackReactionMetadata{
		Channel:   channel,
		Reaction:  reaction,
		Timestamp: timestamp,
	})

	return nil
}

func (s *Slack) PostUpdatedMessage(message slack.Message) error {
	s.UpdatedMessages = append(s.UpdatedMessages, message)

	return nil
}

func (s *Slack) PostWorkflowReaction(log *slog.Logger, event github.Event, channel string, timestamp string) error {
	return s.PostReaction(channel, timestamp, "workflow-reaction")
}
