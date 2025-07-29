package sql

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/navikt/ghep/internal/sql/gensql"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type gooseLogger struct {
	log *slog.Logger
}

func (g *gooseLogger) Fatalf(format string, v ...any) {
	g.log.Error(fmt.Sprintf(format, v...))
}

func (g *gooseLogger) Printf(format string, v ...any) {
	g.log.Info(fmt.Sprintf(format, v...))
}

func New(ctx context.Context, log *slog.Logger, runMigrations bool) (*gensql.Queries, error) {
	url := os.Getenv("PGURL")
	if url == "" {
		return nil, fmt.Errorf("PGURL environment variable is not set")
	}
	url = fmt.Sprintf("%s?default_query_exec_mode=cache_describe", url)

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if runMigrations {
		goose.SetBaseFS(embedMigrations)
		goose.SetLogger(&gooseLogger{log: log})

		if err := goose.SetDialect("postgres"); err != nil {
			return nil, err
		}

		db := stdlib.OpenDBFromPool(pool)

		if err := goose.Up(db, "migrations"); err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	return gensql.New(pool), nil
}
