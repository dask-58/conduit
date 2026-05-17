package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dask-58/conduit/internal/db"
	"github.com/dask-58/conduit/internal/model"
	"github.com/dask-58/conduit/internal/queue"
	"github.com/dask-58/conduit/internal/worker"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/http2"
)

var _ = http2.Server{}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	NewRouter(nil, nil, "test-key").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandlersWithDatabaseAndRedis(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	redisURL := os.Getenv("TEST_REDIS_URL")
	if databaseURL == "" || redisURL == "" {
		t.Skip("TEST_DATABASE_URL and TEST_REDIS_URL are required for integration handlers")
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create postgres pool: %v", err)
	}
	defer pool.Close()

	if err := db.ApplyMigrationFile(ctx, pool, "../../migrations/001_init.sql"); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	if _, err := pool.Exec(ctx, "TRUNCATE delivery_log, idempotency_keys, events, endpoints RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("parse redis url: %v", err)
	}

	rdb := redis.NewClient(redisOptions)
	defer rdb.Close()
	if err := rdb.Del(ctx, queue.JobsKey).Err(); err != nil {
		t.Fatalf("clear redis queue: %v", err)
	}

	queueClient, err := queue.NewClient(ctx, redisURL)
	if err != nil {
		t.Fatalf("create queue client: %v", err)
	}

	var delivered atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delivered.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	queries := db.NewQueries(pool)
	go worker.Start(ctx, queueClient, queries)
	defer func() {
		cancel()
		time.Sleep(100 * time.Millisecond)
		_ = queueClient.Close()
	}()

	apiServer := httptest.NewServer(NewRouter(pool, queueClient, "test-key"))
	defer apiServer.Close()

	created := postEndpoint(t, apiServer.URL, "Deliverable", target.URL)
	deleted := postEndpoint(t, apiServer.URL, "Delete Me", target.URL)

	resp := doJSON(t, http.MethodGet, apiServer.URL+"/endpoints", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list endpoints status: got %d", resp.StatusCode)
	}
	resp.Body.Close()

	req, err := http.NewRequest(http.MethodDelete, apiServer.URL+"/endpoints/"+deleted.ID, nil)
	if err != nil {
		t.Fatalf("create delete request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer test-key")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete endpoint: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete endpoint status: got %d", resp.StatusCode)
	}
	resp.Body.Close()

	event := postIngest(t, apiServer.URL)
	waitFor(t, time.Second, func() bool {
		return delivered.Load() >= 1
	})

	deliveries := waitForDeliveries(t, queries, event.ID)
	if len(deliveries) == 0 {
		t.Fatal("expected delivery trace rows")
	}

	resp = doJSON(t, http.MethodGet, apiServer.URL+"/events?limit=20&offset=0", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list events status: got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, http.MethodGet, apiServer.URL+"/events/"+event.ID+"/deliveries", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get event deliveries status: got %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var apiDeliveries []model.DeliveryLog
	if err := json.NewDecoder(resp.Body).Decode(&apiDeliveries); err != nil {
		t.Fatalf("decode deliveries: %v", err)
	}
	if len(apiDeliveries) == 0 {
		t.Fatal("expected API delivery trace rows")
	}

	if created.ID == "" {
		t.Fatal("created endpoint missing id")
	}
}

func postEndpoint(t *testing.T, baseURL, name, targetURL string) model.Endpoint {
	t.Helper()

	body := map[string]string{"name": name, "url": targetURL}
	resp := doJSON(t, http.MethodPost, baseURL+"/endpoints", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("post endpoint status: got %d", resp.StatusCode)
	}

	var endpoint model.Endpoint
	if err := json.NewDecoder(resp.Body).Decode(&endpoint); err != nil {
		t.Fatalf("decode endpoint: %v", err)
	}

	return endpoint
}

func postIngest(t *testing.T, baseURL string) model.Event {
	t.Helper()

	body := map[string]any{
		"action": "opened",
		"repository": map[string]string{
			"full_name": "dask-58/conduit",
		},
	}
	resp := doJSON(t, http.MethodPost, baseURL+"/ingest", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("post ingest status: got %d", resp.StatusCode)
	}

	var event model.Event
	if err := json.NewDecoder(resp.Body).Decode(&event); err != nil {
		t.Fatalf("decode event: %v", err)
	}

	return event
}

func doJSON(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()

	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode json body: %v", err)
		}
	}

	req, err := http.NewRequest(method, url, &payload)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}

	return resp
}

func waitForDeliveries(t *testing.T, queries *db.Queries, eventID string) []model.DeliveryLog {
	t.Helper()

	var deliveries []model.DeliveryLog
	waitFor(t, time.Second, func() bool {
		var err error
		deliveries, err = queries.GetDeliveriesForEvent(context.Background(), eventID)
		if err != nil {
			t.Fatalf("get deliveries: %v", err)
		}

		return len(deliveries) > 0
	})

	return deliveries
}

func waitFor(t *testing.T, timeout time.Duration, ready func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("timed out waiting for condition")
}
