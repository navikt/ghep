package mock

import (
	"context"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/sql/gensql"
)

type TeamMock struct {
	Members []string
}

func (t *TeamMock) GetTeamMember(_ context.Context, params gensql.GetTeamMemberParams) (string, error) {
	if slices.Contains(t.Members, params.UserLogin) {
		return params.UserLogin, nil
	}

	return "", pgx.ErrNoRows
}
