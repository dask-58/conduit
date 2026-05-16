package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dask-58/conduit/internal/db"
	"github.com/dask-58/conduit/internal/model"
	"github.com/go-chi/chi/v5"
)

func createEndpoint(queries *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input model.Endpoint
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
			return
		}

		if input.Name == "" || input.URL == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and url are required"})
			return
		}

		endpoint, err := queries.InsertEndpoint(r.Context(), input)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "insert endpoint"})
			return
		}

		writeJSON(w, http.StatusCreated, endpoint)
	}
}

func listEndpoints(queries *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		endpoints, err := queries.ListEndpoints(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list endpoints"})
			return
		}

		writeJSON(w, http.StatusOK, endpoints)
	}
}

func deleteEndpoint(queries *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
			return
		}

		if err := queries.DeleteEndpoint(r.Context(), id); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
				return
			}

			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "delete endpoint"})
			return
		}

		writeJSON(w, http.StatusNoContent, nil)
	}
}
