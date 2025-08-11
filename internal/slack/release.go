package slack

import (
	"fmt"

	"github.com/navikt/ghep/internal/github"
)

func CreateReleaseMessage(channel string, event github.Event) *Message {
	releaseType := "release"
	if event.Release.Draft {
		releaseType = "draft release"
	} else if event.Release.Prerelease {
		releaseType = "prerelease"
	}

	text := fmt.Sprintf("%s created a <%s|%s> (`%s`)", event.Sender.ToSlack(), event.Release.URL, releaseType, event.Release.Tag)

	return &Message{
		Channel: channel,
		Text:    text,
		Attachments: []Attachment{
			{
				Text:       event.Release.Body,
				Color:      ColorDefault,
				FooterIcon: neutralGithubIcon,
				Footer:     fmt.Sprintf("<%s|%s>", event.Repository.URL, event.Repository.FullName),
			},
		},
	}
}
