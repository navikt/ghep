package mock

import (
	"context"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/sql/gensql"
)

type Database struct {
	Members            []string
	SlackMessages      []gensql.CreateSlackMessageParams
	Users              []string
	CommitCountUpserts []gensql.UpsertUserCommitCountParams
}

func (m *Database) AddTeamMember(ctx context.Context, params gensql.AddTeamMemberParams) error {
	panic("unimplemented AddTeamMember")
}

func (m *Database) AddTeamRepository(ctx context.Context, params gensql.AddTeamRepositoryParams) error {
	panic("unimplemented AddTeamRepository")
}

func (m *Database) CreateRepository(ctx context.Context, name string) (int32, error) {
	panic("unimplemented CreateRepository")
}

func (m *Database) CreateUser(ctx context.Context, login string) error {
	panic("unimplemented CreateUser")
}

func (m *Database) ExistsUser(ctx context.Context, login string) (bool, error) {
	panic("unimplemented ExistsUser")
}

func (m *Database) ExistsUserCaseInsensitive(_ context.Context, login string) (bool, error) {
	return slices.ContainsFunc(m.Users, func(user string) bool {
		return strings.EqualFold(user, login)
	}), nil
}

func (m *Database) GetRepository(ctx context.Context, name string) (gensql.Repository, error) {
	panic("unimplemented GetRepository")
}

func (m *Database) CreateSlackMessage(ctx context.Context, arg gensql.CreateSlackMessageParams) error {
	m.SlackMessages = append(m.SlackMessages, gensql.CreateSlackMessageParams{
		EventID:  arg.EventID,
		ThreadTs: arg.ThreadTs,
		Channel:  arg.Channel,
		Payload:  arg.Payload,
		TeamSlug: arg.TeamSlug,
	})

	return nil
}

func (m *Database) GetSlackMessage(ctx context.Context, arg gensql.GetSlackMessageParams) (gensql.GetSlackMessageRow, error) {
	for _, m := range m.SlackMessages {
		if arg.EventID == m.EventID {
			return gensql.GetSlackMessageRow{
				ThreadTs: m.ThreadTs,
				Channel:  m.Channel,
				Payload:  m.Payload,
			}, nil
		}
	}

	return gensql.GetSlackMessageRow{}, pgx.ErrNoRows
}

func (m *Database) ListSlackMessagesByEvent(ctx context.Context, arg gensql.ListSlackMessagesByEventParams) ([]gensql.ListSlackMessagesByEventRow, error) {
	rows := []gensql.ListSlackMessagesByEventRow{}

	for _, m := range m.SlackMessages {
		if arg.EventID == m.EventID {
			rows = append(rows, gensql.ListSlackMessagesByEventRow{
				ThreadTs: m.ThreadTs,
				Channel:  m.Channel,
				Payload:  m.Payload,
			})
		}
	}

	if len(rows) == 0 {
		return []gensql.ListSlackMessagesByEventRow{}, pgx.ErrNoRows
	}

	return rows, nil
}

func (m *Database) RemoveTeamMember(ctx context.Context, arg gensql.RemoveTeamMemberParams) error {
	panic("unimplemented RemoveTeamMember")
}

func (m *Database) RemoveTeamRepository(ctx context.Context, arg gensql.RemoveTeamRepositoryParams) error {
	panic("unimplemented RemoveTeamRepository")
}

func (m *Database) UpdateRepository(ctx context.Context, arg gensql.UpdateRepositoryParams) error {
	panic("unimplemented UpdateRepository")
}

func (m *Database) UpsertUserCommitCount(_ context.Context, arg gensql.UpsertUserCommitCountParams) error {
	m.CommitCountUpserts = append(m.CommitCountUpserts, arg)
	return nil
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
