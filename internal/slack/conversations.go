package slack

import (
	"encoding/json"
	"fmt"
)

type conversationsOpenRequest struct {
	Users string `json:"users"`
}

type conversationsOpenResponse struct {
	Response
	Channel struct {
		ID string `json:"id"`
	} `json:"channel"`
}

// OpenDM opens (or re-opens) a direct-message channel with the given Slack user ID
// and returns the channel ID to use with chat.postMessage.
func (c Client) OpenDM(slackUserID string) (string, error) {
	payload, err := json.Marshal(conversationsOpenRequest{Users: slackUserID})
	if err != nil {
		return "", fmt.Errorf("marshalling conversations.open request: %w", err)
	}

	body, err := c.postRequest("conversations.open", payload)
	if err != nil {
		return "", fmt.Errorf("opening DM channel: %w", err)
	}

	var resp conversationsOpenResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return "", fmt.Errorf("unmarshalling conversations.open response: %w", err)
	}

	return resp.Channel.ID, nil
}
