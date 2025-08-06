package integration

import (
	"context"
	"strings"
)

type mockDB struct{}

func (m mockDB) GetUserByEmail(_ context.Context, email string) (string, error) {
	return map[string]string{
		"andre.roaldseth@nav.no":         "androa",
		"kyrre.havik@nav.no":             "Kyrremann",
		"thomas.siegfried.krampl@nav.no": "thokra-nav",
		"frode.sundby@nav.no":            "frodesundby",
		"roger.bjornstad@nav.no":         "rbjornstad",
	}[strings.ToLower(email)], nil
}
