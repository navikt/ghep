package mock

import (
	"context"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/sql/gensql"
)

type Database struct {
	Members []string
}

func (m *Database) GetUserByEmail(_ context.Context, email string) (string, error) {
	return map[string]string{
		"andre.roaldseth@nav.no":         "androa",
		"kyrre.havik@nav.no":             "Kyrremann",
		"thomas.siegfried.krampl@nav.no": "thokra-nav",
		"frode.sundby@nav.no":            "frodesundby",
		"roger.bjornstad@nav.no":         "rbjornstad",
	}[strings.ToLower(email)], nil
}

func (m *Database) GetUserSlackID(_ context.Context, login string) (string, error) {
	return map[string]string{
		"Kyrremann": "U8PL7CR4K",
	}[login], nil
}

func (m *Database) GetTeamMember(_ context.Context, params gensql.GetTeamMemberParams) (string, error) {
	if slices.Contains(m.Members, params.UserLogin) {
		return params.UserLogin, nil
	}

	return "", pgx.ErrNoRows
}
