package api

import (
	"encoding/json"
	"net/http"

	"github.com/dask-58/conduit/internal/db"
)

func ingest(queries *db.Queries) http.HandlerFunc {
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

		writeJSON(w, http.StatusAccepted, event)
	}
}
