package api

import (
	"net/http"
	"strconv"

	"github.com/dask-58/conduit/internal/db"
	"github.com/go-chi/chi/v5"
)

func listEvents(queries *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := parseBoundedInt(r.URL.Query().Get("limit"), 20, 1, 100)
		offset := parseBoundedInt(r.URL.Query().Get("offset"), 0, 0, 100000)

		events, err := queries.ListEvents(r.Context(), limit, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list events"})
			return
		}

		writeJSON(w, http.StatusOK, events)
	}
}

func getEventDeliveries(queries *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		eventID := chi.URLParam(r, "id")
		if eventID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "event id is required"})
			return
		}

		deliveries, err := queries.GetDeliveriesForEvent(r.Context(), eventID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "get event deliveries"})
			return
		}

		writeJSON(w, http.StatusOK, deliveries)
	}
}

func parseBoundedInt(raw string, fallback, min, max int) int {
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	if value < min {
		return min
	}

	if value > max {
		return max
	}

	return value
}
