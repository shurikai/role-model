package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/db"
)

type PositionHandler struct {
	queries *db.Queries
}

func NewPositionHandler(queries *db.Queries) *PositionHandler {
	return &PositionHandler{queries: queries}
}

func (h *PositionHandler) ListByEmployer(w http.ResponseWriter, r *http.Request) {
	employerID, err := uuid.Parse(chi.URLParam(r, "employerID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "employer id must be a valid UUID")
		return
	}

	positions, err := h.queries.GetPositionsByEmployer(r.Context(), db.GetPositionsByEmployerParams{
		EmployerID: employerID,
		UserID:     stubUserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to fetch positions")
		return
	}

	writeJSON(w, http.StatusOK, positions)
}

func (h *PositionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "position id must be a valid UUID")
		return
	}

	position, err := h.queries.GetPosition(r.Context(), db.GetPositionParams{
		ID:     id,
		UserID: stubUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "position not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to fetch position")
		return
	}

	writeJSON(w, http.StatusOK, position)
}
