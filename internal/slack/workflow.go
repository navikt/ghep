package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateWorkflowMessage(channel string, event github.Event) *Message {
	text := fmt.Sprintf(":x: The workflow <%s|#%d %s> triggered by <%s|%s> in the repository <%s|%s> has the status `%s`.", event.Workflow.URL, event.Workflow.RunNumber, event.Workflow.Title, event.Sender.URL, event.Sender.Login, event.Repository.URL, event.Repository.Name, event.Workflow.Conclusion)

	var attachments []Attachment
	if event.Workflow.FailedJob.Name != "" {
		attachments = append(attachments, Attachment{
			Text:  fmt.Sprintf("The job <%s|%s> failed in step `%s`.", event.Workflow.FailedJob.URL, event.Workflow.FailedJob.Name, event.Workflow.FailedJob.Step),
			Color: "#d02434",
		})
	}

	return &Message{
		Channel:     channel,
		Text:        text,
		Attachments: attachments,
	}
}
