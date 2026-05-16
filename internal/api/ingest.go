package api

import (
	"encoding/json"
	"net/http"

	"github.com/dask-58/conduit/internal/db"
	"github.com/dask-58/conduit/internal/model"
	"github.com/dask-58/conduit/internal/queue"
)

func ingest(queries *db.Queries, queueClient *queue.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
			return
		}

		source := r.Header.Get("X-Conduit-Source")
		if source == "" {
			source = "unknown"
		}

		event, err := queries.InsertEvent(r.Context(), source, payload)
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
				Payload:    event.Payload,
			}

			if err := queueClient.Enqueue(r.Context(), job); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "enqueue delivery job"})
				return
			}
		}

		writeJSON(w, http.StatusAccepted, event)
	}
}
