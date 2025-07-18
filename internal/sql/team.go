package sql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/sql/gensql"
)

type TeamQuery interface {
	GetTeamMember(ctx context.Context, arg gensql.GetTeamMemberParams) (string, error)
}

// AddRepositoryToTeam adds a repository to a team, creating the repository if it does not exist.
func AddRepositoryToTeam(ctx context.Context, db *gensql.Queries, team, repositoryName string) error {
	repository, err := db.GetRepository(ctx, repositoryName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			result, err := db.CreateRepository(ctx, repositoryName)
			if err != nil {
				return fmt.Errorf("failed to create repository %s: %w", repositoryName, err)
			}

			repository.ID = result
		} else {
			return err
		}
	}

	return db.AddTeamRepository(ctx, gensql.AddTeamRepositoryParams{
		TeamSlug:     team,
		RepositoryID: repository.ID,
	})
}

// AddMemberToTeam adds a user to a team, creating the user if it does not exist.
func AddMemberToTeam(ctx context.Context, db *gensql.Queries, team, userLogin string) error {
	if _, err := db.GetUser(ctx, userLogin); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := db.CreateUser(ctx, userLogin); err != nil {
				return fmt.Errorf("failed to create user %s: %w", userLogin, err)
			}
		} else {
			return err
		}
	}

	return db.AddTeamMember(ctx, gensql.AddTeamMemberParams{
		TeamSlug:  team,
		UserLogin: userLogin,
	})
}
