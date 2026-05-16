package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dask-58/conduit/internal/model"
	"github.com/redis/go-redis/v9"
)

const JobsKey = "queue:jobs"

type Client struct {
	rdb *redis.Client
}

func NewClient(ctx context.Context, redisURL string) (*Client, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	rdb := redis.NewClient(options)
	if err := rdb.Ping(ctx).Err(); err != nil {
		if closeErr := rdb.Close(); closeErr != nil {
			return nil, fmt.Errorf("ping redis: %w; close redis: %w", err, closeErr)
		}

		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) Enqueue(ctx context.Context, job model.DeliveryJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal delivery job: %w", err)
	}

	if err := c.rdb.LPush(ctx, JobsKey, payload).Err(); err != nil {
		return fmt.Errorf("enqueue delivery job: %w", err)
	}

	return nil
}

func (c *Client) Dequeue(ctx context.Context) (model.DeliveryJob, error) {
	result, err := c.rdb.BRPop(ctx, 0, JobsKey).Result()
	if err != nil {
		return model.DeliveryJob{}, fmt.Errorf("dequeue delivery job: %w", err)
	}

	if len(result) != 2 {
		return model.DeliveryJob{}, fmt.Errorf("dequeue delivery job: expected key and payload, got %d values", len(result))
	}

	var job model.DeliveryJob
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return model.DeliveryJob{}, fmt.Errorf("unmarshal delivery job: %w", err)
	}

	return job, nil
}
