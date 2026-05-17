package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/dask-58/conduit/internal/db"
	"github.com/dask-58/conduit/internal/model"
	"github.com/dask-58/conduit/internal/queue"
)

const (
	MaxWorkers  = 10
	MaxAttempts = 5
)

func Start(ctx context.Context, queueClient *queue.Client, queries *db.Queries) {
	sem := make(chan struct{}, MaxWorkers)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job, err := queueClient.Dequeue(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			if errors.Is(err, queue.ErrNoJob) {
				continue
			}

			slog.Error("dequeue delivery job failed", "error", err)
			continue
		}

		select {
		case <-ctx.Done():
			return
		case sem <- struct{}{}:
		}

		go func(job model.DeliveryJob) {
			defer func() { <-sem }()
			process(ctx, queueClient, queries, job)
		}(job)
	}
}

func process(ctx context.Context, queueClient *queue.Client, queries *db.Queries, job model.DeliveryJob) {
	attrs := []any{
		"event_id", job.EventID,
		"endpoint_id", job.EndpointID,
		"attempt", job.Attempt,
	}

	if job.Attempt == 0 {
		key := fmt.Sprintf("%s:%s", job.EventID, job.EndpointID)
		written, err := queries.WriteIdempotencyKey(ctx, key)
		if err != nil {
			slog.Error("write idempotency key failed", append(attrs, "error", err)...)
			return
		}

		if !written {
			slog.Info("skipping duplicate delivery job", attrs...)
			return
		}
	}

	deliveryLog, err := queries.InsertDeliveryLog(ctx, job.EventID, job.EndpointID, job.Attempt)
	if err != nil {
		slog.Error("insert delivery log failed", append(attrs, "error", err)...)
		return
	}

	endpoint, err := queries.GetEndpoint(ctx, job.EndpointID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			if markErr := queries.MarkExhausted(ctx, deliveryLog.ID, nil); markErr != nil {
				slog.Error("mark delivery exhausted failed", append(attrs, "error", markErr)...)
			}

			slog.Error("endpoint not found for delivery job", attrs...)
			return
		}

		slog.Error("get endpoint failed", append(attrs, "error", err)...)
		return
	}

	statusCode, err := Deliver(job, endpoint)
	responseCode := responseCodePtr(statusCode)
	if err == nil {
		if updateErr := queries.UpdateDeliveryStatus(ctx, deliveryLog.ID, model.StatusSuccess, responseCode, nil); updateErr != nil {
			slog.Error("update successful delivery failed", append(attrs, "error", updateErr, "response_code", statusCode)...)
			return
		}

		slog.Info("delivery succeeded", append(attrs, "response_code", statusCode)...)
		return
	}

	if job.Attempt >= MaxAttempts {
		if markErr := queries.MarkExhausted(ctx, deliveryLog.ID, responseCode); markErr != nil {
			slog.Error("mark delivery exhausted failed", append(attrs, "error", markErr, "response_code", statusCode)...)
			return
		}

		slog.Error("delivery exhausted", append(attrs, "error", err, "response_code", statusCode)...)
		return
	}

	delay := Backoff(job.Attempt)
	nextRetryAt := time.Now().Add(delay)
	if updateErr := queries.UpdateDeliveryStatus(ctx, deliveryLog.ID, model.StatusFailed, responseCode, &nextRetryAt); updateErr != nil {
		slog.Error("update failed delivery failed", append(attrs, "error", updateErr, "response_code", statusCode)...)
		return
	}

	slog.Error("delivery failed, retrying", append(attrs, "error", err, "response_code", statusCode, "retry_in", delay)...)

	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}

	nextJob := job
	nextJob.Attempt++
	if enqueueErr := queueClient.Enqueue(ctx, nextJob); enqueueErr != nil {
		slog.Error("enqueue retry delivery job failed", append(attrs, "error", enqueueErr, "next_attempt", nextJob.Attempt)...)
		return
	}

	slog.Info("retry delivery job enqueued", append(attrs, "next_attempt", nextJob.Attempt)...)
}

func responseCodePtr(statusCode int) *int {
	if statusCode == 0 {
		return nil
	}

	return &statusCode
}
