package slack

import (
	"bytes"
	"embed"
	"fmt"
	"net/http"
	"text/template"
	"time"
)

var (
	//go:embed templates/*.tmpl
	templates  embed.FS
	commitTmpl *template.Template
)

type Client struct {
	httpClient *http.Client
	token      string
}

func New(token string) (Client, error) {
	if token == "" {
		return Client{}, fmt.Errorf("missing Slack token")
	}

	var err error
	commitTmpl, err = template.ParseFS(templates, "templates/commit.tmpl")
	if err != nil {
		return Client{}, err
	}

	return Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		token: token,
	}, nil
}

func (c Client) PostMessage(payload []byte) error {
	body := bytes.NewReader(payload)

	req, err := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", body)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("error posting message to Slack(%v): %v", resp.Status, resp.Body)
	}

	return nil
}
