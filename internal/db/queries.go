package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dask-58/conduit/internal/model"
	"github.com/jackc/pgx/v5"
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

func (q *Queries) GetEndpoint(ctx context.Context, id string) (model.Endpoint, error) {
	const query = `
		SELECT id::text, name, url, COALESCE(secret, ''), created_at
		FROM endpoints
		WHERE id = $1
	`

	var endpoint model.Endpoint
	if err := q.pool.QueryRow(ctx, query, id).Scan(
		&endpoint.ID,
		&endpoint.Name,
		&endpoint.URL,
		&endpoint.Secret,
		&endpoint.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Endpoint{}, ErrNotFound
		}

		return model.Endpoint{}, fmt.Errorf("get endpoint: %w", err)
	}

	return endpoint, nil
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

func (q *Queries) InsertDeliveryLog(ctx context.Context, eventID, endpointID string, attempt int) (model.DeliveryLog, error) {
	const query = `
		INSERT INTO delivery_log (event_id, endpoint_id, status, attempt)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, event_id::text, endpoint_id::text, status, attempt,
			next_retry_at, response_code, delivered_at, created_at, updated_at
	`

	var log model.DeliveryLog
	if err := q.pool.QueryRow(ctx, query, eventID, endpointID, model.StatusPending, attempt).Scan(
		&log.ID,
		&log.EventID,
		&log.EndpointID,
		&log.Status,
		&log.Attempt,
		&log.NextRetryAt,
		&log.ResponseCode,
		&log.DeliveredAt,
		&log.CreatedAt,
		&log.UpdatedAt,
	); err != nil {
		return model.DeliveryLog{}, fmt.Errorf("insert delivery log: %w", err)
	}

	return log, nil
}

func (q *Queries) UpdateDeliveryStatus(ctx context.Context, id string, status model.DeliveryStatus, responseCode *int, nextRetryAt *time.Time) error {
	const query = `
		UPDATE delivery_log
		SET status = $2,
			response_code = $3,
			next_retry_at = $4,
			delivered_at = CASE WHEN $2 = 'success' THEN now() ELSE delivered_at END,
			updated_at = now()
		WHERE id = $1
	`

	tag, err := q.pool.Exec(ctx, query, id, status, responseCode, nextRetryAt)
	if err != nil {
		return fmt.Errorf("update delivery status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (q *Queries) MarkExhausted(ctx context.Context, id string, responseCode *int) error {
	const query = `
		UPDATE delivery_log
		SET status = $2,
			response_code = $3,
			next_retry_at = NULL,
			updated_at = now()
		WHERE id = $1
	`

	tag, err := q.pool.Exec(ctx, query, id, model.StatusExhausted, responseCode)
	if err != nil {
		return fmt.Errorf("mark exhausted: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (q *Queries) WriteIdempotencyKey(ctx context.Context, key string) (bool, error) {
	const query = `
		INSERT INTO idempotency_keys (key)
		VALUES ($1)
		ON CONFLICT DO NOTHING
	`

	tag, err := q.pool.Exec(ctx, query, key)
	if err != nil {
		return false, fmt.Errorf("write idempotency key: %w", err)
	}

	return tag.RowsAffected() == 1, nil
}

func (q *Queries) ListEvents(ctx context.Context, limit, offset int) ([]model.EventSummary, error) {
	const query = `
		SELECT
			e.id::text,
			e.source,
			e.payload,
			e.received_at,
			COUNT(dl.id)::int AS total_deliveries,
			COUNT(dl.id) FILTER (WHERE dl.status = 'pending')::int AS pending,
			COUNT(dl.id) FILTER (WHERE dl.status = 'success')::int AS success,
			COUNT(dl.id) FILTER (WHERE dl.status = 'failed')::int AS failed,
			COUNT(dl.id) FILTER (WHERE dl.status = 'exhausted')::int AS exhausted,
			CASE
				WHEN COUNT(dl.id) = 0 THEN 'pending'
				WHEN COUNT(dl.id) FILTER (WHERE dl.status = 'exhausted') > 0 THEN 'exhausted'
				WHEN COUNT(dl.id) FILTER (WHERE dl.status = 'pending') > 0 THEN 'pending'
				WHEN COUNT(dl.id) FILTER (WHERE dl.status = 'failed') > 0 THEN 'retrying'
				ELSE 'success'
			END AS summary_status
		FROM events e
		LEFT JOIN delivery_log dl ON dl.event_id = e.id
		GROUP BY e.id
		ORDER BY e.received_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := q.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	events := make([]model.EventSummary, 0)
	for rows.Next() {
		var event model.EventSummary
		if err := rows.Scan(
			&event.ID,
			&event.Source,
			&event.Payload,
			&event.ReceivedAt,
			&event.TotalDeliveries,
			&event.Pending,
			&event.Success,
			&event.Failed,
			&event.Exhausted,
			&event.SummaryStatus,
		); err != nil {
			return nil, fmt.Errorf("scan event summary: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}

func (q *Queries) GetDeliveriesForEvent(ctx context.Context, eventID string) ([]model.DeliveryLog, error) {
	const query = `
		SELECT id::text, event_id::text, endpoint_id::text, status, attempt,
			next_retry_at, response_code, delivered_at, created_at, updated_at
		FROM delivery_log
		WHERE event_id = $1
		ORDER BY created_at ASC, attempt ASC
	`

	rows, err := q.pool.Query(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("get deliveries for event: %w", err)
	}
	defer rows.Close()

	deliveries := make([]model.DeliveryLog, 0)
	for rows.Next() {
		var delivery model.DeliveryLog
		if err := rows.Scan(
			&delivery.ID,
			&delivery.EventID,
			&delivery.EndpointID,
			&delivery.Status,
			&delivery.Attempt,
			&delivery.NextRetryAt,
			&delivery.ResponseCode,
			&delivery.DeliveredAt,
			&delivery.CreatedAt,
			&delivery.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan delivery log: %w", err)
		}

		deliveries = append(deliveries, delivery)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delivery logs: %w", err)
	}

	return deliveries, nil
}
