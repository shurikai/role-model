package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/db"
)

// TODO: replace with authenticated user once auth is implemented.
// Single-user stub — matches the seeded user_id.
var stubUserID = uuid.MustParse("a0000000-0000-0000-0000-000000000001")

type EmployerHandler struct {
	queries *db.Queries
}

func NewEmployerHandler(queries *db.Queries) *EmployerHandler {
	return &EmployerHandler{queries: queries}
}

func (h *EmployerHandler) List(w http.ResponseWriter, r *http.Request) {
	employers, err := h.queries.GetEmployers(r.Context(), stubUserID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch employers")
		return
	}

	writeJSON(w, http.StatusOK, employers)
}

func (h *EmployerHandler) Get(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "employer id must be a valid UUID")
		return
	}

	employer, err := h.queries.GetEmployer(r.Context(), db.GetEmployerParams{
		ID:     id,
		UserID: stubUserID,
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
