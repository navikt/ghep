package slack

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

const (
	slackApi = "https://slack.com/api"
)

type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type responseData struct {
	Ok               bool      `json:"ok"`
	Error            string    `json:"error"`
	Nedded           string    `json:"needed"`
	Provided         string    `json:"provided"`
	Warn             string    `json:"warning"`
	Channels         []Channel `json:"channels"`
	TimeStamp        string    `json:"ts"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
	Message struct {
		Reactions []struct {
			Name string `json:"name"`
		} `json:"reactions"`
	} `json:"message"`
}

type Client struct {
	log        *slog.Logger
	httpClient *http.Client
	token      string
	templates  map[string]template.Template
}

func (c Client) CommitTmpl() template.Template {
	return c.templates["commit"]
}

func (c Client) IssueTmpl() template.Template {
	return c.templates["issue"]
}

func (c Client) PullRequestTmpl() template.Template {
	return c.templates["pull"]
}

func (c Client) RemovedTmpl() template.Template {
	return c.templates["removed"]
}

func (c Client) RenamedTmpl() template.Template {
	return c.templates["renamed"]
}

func (c Client) TeamTmpl() template.Template {
	return c.templates["team"]
}

func (c Client) WorkflowTmpl() template.Template {
	return c.templates["workflow"]
}

func New(log *slog.Logger, token string) (Client, error) {
	if token == "" {
		return Client{}, fmt.Errorf("missing Slack token")
	}

	templates, err := ParseMessageTemplates()
	if err != nil {
		return Client{}, err
	}

	client := Client{
		log: log,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		token:     token,
		templates: templates,
	}

	return client, nil
}

func ParseMessageTemplates() (map[string]template.Template, error) {
	templates := map[string]template.Template{}

	files, err := templatesFS.ReadDir("templates")
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		tmpl, err := template.ParseFS(templatesFS, "templates/"+file.Name())
		if err != nil {
			return nil, err
		}

		name := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		templates[name] = *tmpl
	}

	return templates, nil
}

func (c Client) request(apiMethod string, payload []byte) (*responseData, error) {
	req, err := http.NewRequest("POST", slackApi+"/"+apiMethod, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpDoWithRetry(req, 3)
	if err != nil {
		return nil, fmt.Errorf("giving up after 3 retries: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var slackResp responseData
	if err := json.Unmarshal([]byte(body), &slackResp); err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return &slackResp, fmt.Errorf("non 200 status code(%v): %v", resp.StatusCode, slackResp.Error)
	}

	if !slackResp.Ok {
		return &slackResp, fmt.Errorf("non OK: %v (needed=%s, provded=%s)", slackResp.Error, slackResp.Nedded, slackResp.Provided)
	}

	if slackResp.Warn != "" {
		c.log.Info("got a warning", "warn", slackResp.Warn)
	}

	return &slackResp, nil
}

func (c Client) httpDoWithRetry(req *http.Request, retries int) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if retries == 0 {
			return nil, err
		}

		time.Sleep(1 * time.Second)

		return c.httpDoWithRetry(req, retries-1)
	}

	return resp, nil
}
