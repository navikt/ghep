package mock

import (
	"encoding/json"
	"log/slog"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
)

type Slack struct {
	Messages []slack.Message
}

func (s Slack) EnsureChannels(teams map[string]github.Team) error {
	panic("unimplemented EnsureChannels")
}

func (s Slack) GetReactions(channel string, timestamp string) ([]string, error) {
	panic("unimplemented GetReactions")
}

func (s Slack) JoinChannel(channel string) error {
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

func (s Slack) PostReaction(channel string, timestamp string, reaction string) error {
	panic("unimplemented PostReaction")
}

func (s Slack) PostUpdatedMessage(message slack.Message) error {
	panic("unimplemented PostUpdatedMessage")
}

func (s Slack) PostWorkflowReaction(log *slog.Logger, event github.Event, channel string, timestamp string) error {
	panic("unimplemented PostWorkflowReaction")
}
