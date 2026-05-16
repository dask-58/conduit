package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dask-58/conduit/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Queries struct {
	pool *pgxpool.Pool
}

func NewQueries(pool *pgxpool.Pool) *Queries {
	return &Queries{pool: pool}
}

func (q *Queries) InsertEvent(ctx context.Context, source string, payload json.RawMessage) (model.Event, error) {
	const query = `
		INSERT INTO events (source, payload)
		VALUES ($1, $2)
		RETURNING id::text, source, payload, received_at
	`

	var event model.Event
	if err := q.pool.QueryRow(ctx, query, source, payload).Scan(
		&event.ID,
		&event.Source,
		&event.Payload,
		&event.ReceivedAt,
	); err != nil {
		return model.Event{}, fmt.Errorf("insert event: %w", err)
	}

	return event, nil
}

func (q *Queries) InsertEndpoint(ctx context.Context, endpoint model.Endpoint) (model.Endpoint, error) {
	const query = `
		INSERT INTO endpoints (name, url, secret)
		VALUES ($1, $2, NULLIF($3, ''))
		RETURNING id::text, name, url, COALESCE(secret, ''), created_at
	`

	var inserted model.Endpoint
	if err := q.pool.QueryRow(ctx, query, endpoint.Name, endpoint.URL, endpoint.Secret).Scan(
		&inserted.ID,
		&inserted.Name,
		&inserted.URL,
		&inserted.Secret,
		&inserted.CreatedAt,
	); err != nil {
		return model.Endpoint{}, fmt.Errorf("insert endpoint: %w", err)
	}

	return inserted, nil
}

func (q *Queries) ListEndpoints(ctx context.Context) ([]model.Endpoint, error) {
	const query = `
		SELECT id::text, name, url, COALESCE(secret, ''), created_at
		FROM endpoints
		ORDER BY created_at DESC
	`

	rows, err := q.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}
	defer rows.Close()

	endpoints := make([]model.Endpoint, 0)
	for rows.Next() {
		var endpoint model.Endpoint
		if err := rows.Scan(
			&endpoint.ID,
			&endpoint.Name,
			&endpoint.URL,
			&endpoint.Secret,
			&endpoint.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan endpoint: %w", err)
		}

		endpoints = append(endpoints, endpoint)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate endpoints: %w", err)
	}

	return endpoints, nil
}

func (q *Queries) DeleteEndpoint(ctx context.Context, id string) error {
	const query = `DELETE FROM endpoints WHERE id = $1`

	tag, err := q.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete endpoint: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
