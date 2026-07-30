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

func (m *Database) AddTeamMember(ctx context.Context, params gensql.AddTeamMemberParams) error {
	panic("unimplemented")
}

func (m *Database) AddTeamRepository(ctx context.Context, params gensql.AddTeamRepositoryParams) error {
	panic("unimplemented")
}

func (m *Database) CreateRepository(ctx context.Context, name string) (int32, error) {
	panic("unimplemented")
}

func (m *Database) CreateUser(ctx context.Context, login string) error {
	panic("unimplemented")
}

func (m *Database) ExistsUser(ctx context.Context, login string) (bool, error) {
	panic("unimplemented")
}

func (m *Database) GetRepository(ctx context.Context, name string) (gensql.Repository, error) {
	panic("unimplemented")
}

func (m *Database) CreateSlackMessage(ctx context.Context, arg gensql.CreateSlackMessageParams) error {
	panic("unimplemented")
}

func (m *Database) GetSlackMessage(ctx context.Context, arg gensql.GetSlackMessageParams) (gensql.GetSlackMessageRow, error) {
	panic("unimplemented")
}

func (m *Database) ListSlackMessagesByEvent(ctx context.Context, arg gensql.ListSlackMessagesByEventParams) ([]gensql.ListSlackMessagesByEventRow, error) {
	panic("unimplemented")
}

func (m *Database) RemoveTeamMember(ctx context.Context, arg gensql.RemoveTeamMemberParams) error {
	panic("unimplemented")
}

func (m *Database) RemoveTeamRepository(ctx context.Context, arg gensql.RemoveTeamRepositoryParams) error {
	panic("unimplemented")
}

func (m *Database) UpdateRepository(ctx context.Context, arg gensql.UpdateRepositoryParams) error {
	panic("unimplemented")
}

func (m *Database) UpsertUserCommitCount(ctx context.Context, arg gensql.UpsertUserCommitCountParams) error {
	panic("unimplemented")
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
