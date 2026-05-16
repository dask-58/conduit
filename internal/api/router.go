package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dask-58/conduit/internal/db"
	"github.com/dask-58/conduit/internal/queue"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(pool *pgxpool.Pool, queueClient *queue.Client, apiKey string) http.Handler {
	r := chi.NewRouter()
	queries := db.NewQueries(pool)

	r.Get("/healthz", healthz)

	r.Group(func(r chi.Router) {
		r.Use(authorizeBearer(apiKey))

		r.Post("/ingest", ingest(queries, queueClient))
		r.Route("/endpoints", func(r chi.Router) {
			r.Get("/", listEndpoints(queries))
			r.Post("/", createEndpoint(queries))
			r.Delete("/{id}", deleteEndpoint(queries))
		})
	})

	return r
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func authorizeBearer(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token != apiKey {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if value == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
