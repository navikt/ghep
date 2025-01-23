package slack

import (
	"fmt"
)

func (c Client) PostMessage(payload []byte) (string, error) {
	resp, err := c.postRequest("chat.postMessage", payload)
	if err != nil {
		return "", fmt.Errorf("error posting message to Slack: %v", err)
	}

	return resp.TimeStamp, nil
}

func (c Client) PostUpdatedMessage(payload []byte) (string, error) {
	resp, err := c.postRequest("chat.update", payload)
	if err != nil {
		return "", fmt.Errorf("error updating message in Slack: %v", err)
	}

	return resp.TimeStamp, nil
}
