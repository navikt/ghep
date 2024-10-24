package integration

import (
	"github.com/navikt/ghep/internal/github"
)

type mockHub struct {
}

func (m mockHub) GetUserByEmail(email string) (github.User, error) {
	return map[string]github.User{
		"andre.roaldseth@nav.no": {
			Login: "androa",
			URL:   "https://github.com/androa",
			Name:  "André Roaldseth",
		},
		"Kyrre.Havik@nav.no": {
			Login: "Kyrremann",
			URL:   "https://github.com/Kyrremann",
			Name:  "Kyrre Havik",
		},
	}[email], nil
}
