package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/httputil"
	"github.com/shurikai/role-model/internal/db"
)

type EmployerHandler struct {
	queries *db.Queries
}

func NewEmployerHandler(queries *db.Queries) *EmployerHandler {
	return &EmployerHandler{queries: queries}
}

type createEmployerRequest struct {
	Name     string  `json:"name"`
	Industry *string `json:"industry"`
	Notes    *string `json:"notes"`
}

type updateEmployerRequest struct {
	Name     string  `json:"name"`
	Industry *string `json:"industry"`
	Notes    *string `json:"notes"`
}

func (h *EmployerHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	employers, err := h.queries.GetEmployers(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch employers")
		return
	}

	if employers == nil {
		employers = []db.Employer{}
	}

	httputil.WriteJSON(w, http.StatusOK, employers)
}

func (h *EmployerHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "employer id must be a valid UUID")
		return
	}

	employer, err := h.queries.GetEmployer(r.Context(), db.GetEmployerParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "employer not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch employer")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, employer)
}

func (h *EmployerHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req createEmployerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}

	employer, err := h.queries.CreateEmployer(r.Context(), db.CreateEmployerParams{
		ID:       uuid.New(),
		UserID:   userID,
		Name:     req.Name,
		Industry: req.Industry,
		Notes:    req.Notes,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create employer")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, employer)
}

func (h *EmployerHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "employer id must be a valid UUID")
		return
	}

	var req updateEmployerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}

	employer, err := h.queries.UpdateEmployer(r.Context(), db.UpdateEmployerParams{
		ID:       id,
		UserID:   userID,
		Name:     req.Name,
		Industry: req.Industry,
		Notes:    req.Notes,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "employer not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update employer")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, employer)
}

func (h *EmployerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "employer id must be a valid UUID")
		return
	}

	// Guard: refuse if the employer has positions.
	count, err := h.queries.CountPositionsByEmployer(r.Context(), db.CountPositionsByEmployerParams{
		EmployerID: id,
		UserID:     userID,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to check employer dependents")
		return
	}
	if count > 0 {
		httputil.WriteError(w, http.StatusConflict, "has_dependents", "employer has positions and cannot be deleted")
		return
	}

	if err := h.queries.DeleteEmployer(r.Context(), db.DeleteEmployerParams{
		ID:     id,
		UserID: userID,
	}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete employer")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
