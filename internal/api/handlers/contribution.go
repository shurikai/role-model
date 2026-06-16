package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/db"
)

type ContributionHandler struct {
	queries *db.Queries
}

func NewContributionHandler(queries *db.Queries) *ContributionHandler {
	return &ContributionHandler{queries: queries}
}

func (h *ContributionHandler) ListByPosition(w http.ResponseWriter, r *http.Request) {
	positionID, err := uuid.Parse(chi.URLParam(r, "positionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "position id must be a valid UUID")
		return
	}

	contributions, err := h.queries.GetContributionsByPosition(r.Context(), db.GetContributionsByPositionParams{
		PositionID: positionID,
		UserID:     stubUserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to fetch contributions")
		return
	}

	writeJSON(w, http.StatusOK, contributions)
}

func (h *ContributionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "contribution id must be a valid UUID")
		return
	}

	contribution, err := h.queries.GetContribution(r.Context(), db.GetContributionParams{
		ID:     id,
		UserID: stubUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "contribution not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to fetch contribution")
		return
	}

	writeJSON(w, http.StatusOK, contribution)
}
