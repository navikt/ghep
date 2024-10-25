package slack

import (
	"fmt"
)

func (c Client) PostMessage(payload []byte) (string, error) {
	resp, err := c.request("chat.postMessage", payload)
	if err != nil {
		return "", fmt.Errorf("error posting message to Slack: %v", err)
	}

	return resp.TimeStamp, nil
}
