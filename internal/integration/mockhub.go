package integration

import (
	"github.com/navikt/ghep/internal/github"
)

type mockHub struct{}

func (m mockHub) GetUserByEmail(email string) (*github.User, error) {
	return map[string]*github.User{
		"andre.roaldseth@nav.no": {
			Login: "androa",
		},
		"Kyrre.Havik@nav.no": {
			Login: "Kyrremann",
		},
	}[email], nil
}
