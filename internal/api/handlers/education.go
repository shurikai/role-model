package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/httputil"
)

type EducationHandler struct {
	queries *db.Queries
}

func NewEducationHandler(queries *db.Queries) *EducationHandler {
	return &EducationHandler{queries: queries}
}

type educationRequest struct {
	Institution  string  `json:"institution"`
	Degree       *string `json:"degree"`
	FieldOfStudy *string `json:"field_of_study"`
	StartedOn    *string `json:"started_on"` // nullable date
	EndedOn      *string `json:"ended_on"`   // nullable date
	Notes        *string `json:"notes"`
}

func (h *EducationHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req educationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Institution == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "institution is required")
		return
	}

	startedOn, err := httputil.ParseNullableDate(req.StartedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "started_on must be YYYY-MM-DD or null")
		return
	}
	endedOn, err := httputil.ParseNullableDate(req.EndedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "ended_on must be YYYY-MM-DD or null")
		return
	}

	edu, err := h.queries.CreateEducation(r.Context(), db.CreateEducationParams{
		ID:           uuid.New(),
		UserID:       userID,
		Institution:  req.Institution,
		Degree:       req.Degree,
		FieldOfStudy: req.FieldOfStudy,
		StartedOn:    startedOn,
		EndedOn:      endedOn,
		Notes:        req.Notes,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create education")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, edu)
}

func (h *EducationHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "education id must be a valid UUID")
		return
	}

	var req educationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Institution == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "institution is required")
		return
	}

	startedOn, err := httputil.ParseNullableDate(req.StartedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "started_on must be YYYY-MM-DD or null")
		return
	}
	endedOn, err := httputil.ParseNullableDate(req.EndedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "ended_on must be YYYY-MM-DD or null")
		return
	}

	edu, err := h.queries.UpdateEducation(r.Context(), db.UpdateEducationParams{
		ID:           id,
		UserID:       userID,
		Institution:  req.Institution,
		Degree:       req.Degree,
		FieldOfStudy: req.FieldOfStudy,
		StartedOn:    startedOn,
		EndedOn:      endedOn,
		Notes:        req.Notes,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "education not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update education")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, edu)
}

func (h *EducationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "education id must be a valid UUID")
		return
	}

	rows, err := h.queries.DeleteEducation(r.Context(), db.DeleteEducationParams{ID: id, UserID: userID})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete education")
		return
	}
	if rows == 0 {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "education not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
