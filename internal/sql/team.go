package sql

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/ghep/internal/sql/gensql"
)

// AddRepositoryToTeam adds a repository to a team, creating the repository if it does not exist.
func AddRepositoryToTeam(ctx context.Context, db *gensql.Queries, team, repositoryName string) error {
	repository, err := db.GetRepository(ctx, repositoryName)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		result, err := db.CreateRepository(ctx, repositoryName)
		if err != nil {
			return fmt.Errorf("failed to create repository %s: %w", repositoryName, err)
		}

		repository.ID = result
	}

	return db.AddTeamRepository(ctx, gensql.AddTeamRepositoryParams{
		TeamSlug:     team,
		RepositoryID: repository.ID,
	})
}

// AddMemberToTeam adds a user to a team, creating the user if it does not exist.
func AddMemberToTeam(ctx context.Context, db *gensql.Queries, team, userLogin string) error {
	exists, err := db.ExistsUser(ctx, userLogin)
	if err != nil {
		return err
	}

	if !exists {
		if err := db.CreateUser(ctx, userLogin); err != nil {
			return fmt.Errorf("failed to create user %s: %w", userLogin, err)
		}
	}

	return db.AddTeamMember(ctx, gensql.AddTeamMemberParams{
		TeamSlug:  team,
		UserLogin: userLogin,
	})
}

func RemoveRepositoriesNotBelongingToTeam(ctx context.Context, db *gensql.Queries, team string, repositories []string) error {
	currentRepositories, err := db.ListTeamRepositories(ctx, team)
	if err != nil {
		return err
	}

	for _, repository := range currentRepositories {
		exists := slices.Contains(repositories, repository.Name)
		if !exists {
			db.RemoveTeamRepository(ctx, gensql.RemoveTeamRepositoryParams{
				TeamSlug: team,
				Name:     repository.Name,
			})
		}
	}

	return nil
}
