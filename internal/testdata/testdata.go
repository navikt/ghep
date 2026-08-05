package testdata

import (
	"embed"
	"encoding/json"

	"github.com/navikt/ghep/internal/github"
)

//go:embed events
var files embed.FS

func AsEvent(file string) (github.Event, error) {
	goldenfile, err := files.ReadFile("events/" + file)
	if err != nil {
		return github.Event{}, err
	}

	var event github.Event
	err = json.Unmarshal(goldenfile, &event)
	return event, err
}
