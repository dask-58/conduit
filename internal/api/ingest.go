package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/dask-58/conduit/internal/db"
	"github.com/dask-58/conduit/internal/model"
	"github.com/dask-58/conduit/internal/queue"
)

func ingest(queries *db.Queries, queueClient *queue.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
			return
		}

		if !json.Valid(payload) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
			return
		}

		source := r.Header.Get("X-Github-Event")
		if source == "" {
			source = "unknown"
		}

		event, err := queries.InsertEvent(r.Context(), source, json.RawMessage(payload))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "insert event"})
			return
		}

		endpoints, err := queries.ListEndpoints(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list endpoints"})
			return
		}

		for _, endpoint := range endpoints {
			job := model.DeliveryJob{
				EventID:    event.ID,
				EndpointID: endpoint.ID,
				Attempt:    0,
				Payload:    json.RawMessage(payload),
			}

			if err := queueClient.Enqueue(r.Context(), job); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "enqueue delivery job"})
				return
			}
		}

		writeJSON(w, http.StatusAccepted, event)
	}
}
