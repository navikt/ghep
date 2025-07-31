package slack

import (
	"encoding/json"
	"fmt"
)

func (c Client) PostMessage(payload []byte) (*MessageResponse, error) {
	body, err := c.postRequest("chat.postMessage", payload)
	if err != nil {
		return nil, fmt.Errorf("error posting message to Slack: %v", err)
	}

	var resp MessageResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c Client) PostUpdatedMessage(message Message) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("error marshalling message: %v", err)
	}

	body, err := c.postRequest("chat.update", payload)
	if err != nil {
		return fmt.Errorf("error updating message in Slack: %v", err)
	}

	var resp MessageResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return err
	}

	return nil
}
