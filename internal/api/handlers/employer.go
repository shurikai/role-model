package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/api/middleware"
	"github.com/shurikai/role-model/internal/db"
)

type EmployerHandler struct {
	queries *db.Queries
}

func NewEmployerHandler(queries *db.Queries) *EmployerHandler {
	return &EmployerHandler{queries: queries}
}

func (h *EmployerHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	employers, err := h.queries.GetEmployers(r.Context(), userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch employers")
		return
	}

	if employers == nil {
		employers = []db.Employer{}
	}

	writeJSON(w, http.StatusOK, employers)
}

func (h *EmployerHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "employer id must be a valid UUID")
		return
	}

	employer, err := h.queries.GetEmployer(r.Context(), db.GetEmployerParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, http.StatusNotFound, "not_found", "employer not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch employer")
		return
	}

	writeJSON(w, http.StatusOK, employer)
}
