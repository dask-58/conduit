package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPostgresPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}

func ApplyMigrationFile(ctx context.Context, pool *pgxpool.Pool, path string) error {
	sql, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", path, err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Conn().PgConn().Exec(ctx, string(sql)).ReadAll(); err != nil {
		return fmt.Errorf("execute migration %s: %w", path, err)
	}

	return nil
}
