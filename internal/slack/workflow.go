package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateWorkflowMessage(channel string, event github.Event) *Message {
	text := fmt.Sprintf(":x: %s has a workflow with status `%s`, triggered by %s.\n<%s|#%d %s>", event.Repository.ToSlack(), event.Workflow.Conclusion, event.Sender.ToSlack(), event.Workflow.URL, event.Workflow.RunNumber, event.Workflow.Title)

	var attachments []Attachment
	if event.Workflow.FailedJob.Name != "" {
		attachments = append(attachments, Attachment{
			Text:       fmt.Sprintf("The job <%s|%s>[%s] failed in step `%s`.", event.Workflow.FailedJob.URL, event.Workflow.FailedJob.Name, event.Workflow.HeadBranch, event.Workflow.FailedJob.Step),
			Color:      "#d02434",
			Footer:     fmt.Sprintf("<%s|%s>", event.Repository.URL, event.Repository.FullName),
			FooterIcon: "https://slack.github.com/static/img/favicon-neutral.png",
		})
	}

	return &Message{
		Channel:     channel,
		Text:        text,
		Attachments: attachments,
	}
}
