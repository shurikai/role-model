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

type PositionHandler struct {
	queries *db.Queries
}

func NewPositionHandler(queries *db.Queries) *PositionHandler {
	return &PositionHandler{queries: queries}
}

type createPositionRequest struct {
	EmployerID       uuid.UUID `json:"employer_id"`
	Title            string    `json:"title"`
	IndustryLevel    *string   `json:"industry_level"`
	IndustryRole     *string   `json:"industry_role"`
	Location         *string   `json:"location"`
	LevelRationale   *string   `json:"level_rationale"`
	StartedOn        string    `json:"started_on"` // "YYYY-MM-DD"
	EndedOn          *string   `json:"ended_on"`   // nullable
	ContextNarrative *string   `json:"context_narrative"`
	SortOrder        int32     `json:"sort_order"`
}

type updatePositionRequest struct {
	Title            string  `json:"title"`
	IndustryLevel    *string `json:"industry_level"`
	IndustryRole     *string `json:"industry_role"`
	Location         *string `json:"location"`
	LevelRationale   *string `json:"level_rationale"`
	StartedOn        string  `json:"started_on"`
	EndedOn          *string `json:"ended_on"`
	ContextNarrative *string `json:"context_narrative"`
	SortOrder        int32   `json:"sort_order"`
}

func (h *PositionHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req createPositionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Title == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "title is required")
		return
	}
	if req.EmployerID == uuid.Nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "employer_id is required")
		return
	}

	// Parent-ownership check: the employer must exist AND belong to this user.
	if _, err := h.queries.GetEmployer(r.Context(), db.GetEmployerParams{
		ID:     req.EmployerID,
		UserID: userID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "employer_not_found", "employer not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify employer")
		return
	}

	startedOn, err := httputil.ParseDate(req.StartedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "started_on must be YYYY-MM-DD")
		return
	}
	endedOn, err := httputil.ParseNullableDate(req.EndedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "ended_on must be YYYY-MM-DD or null")
		return
	}

	position, err := h.queries.CreatePosition(r.Context(), db.CreatePositionParams{
		ID:               uuid.New(),
		UserID:           userID,
		EmployerID:       req.EmployerID,
		Title:            req.Title,
		IndustryLevel:    req.IndustryLevel,
		IndustryRole:     req.IndustryRole,
		Location:         req.Location,
		LevelRationale:   req.LevelRationale,
		StartedOn:        startedOn,
		EndedOn:          endedOn,
		ContextNarrative: req.ContextNarrative,
		SortOrder:        req.SortOrder,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create position")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, position)
}

func (h *PositionHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "position id must be a valid UUID")
		return
	}

	var req updatePositionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Title == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "title is required")
		return
	}

	startedOn, err := httputil.ParseDate(req.StartedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "started_on must be YYYY-MM-DD")
		return
	}
	endedOn, err := httputil.ParseNullableDate(req.EndedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "ended_on must be YYYY-MM-DD or null")
		return
	}

	position, err := h.queries.UpdatePosition(r.Context(), db.UpdatePositionParams{
		ID:               id,
		UserID:           userID,
		Title:            req.Title,
		IndustryLevel:    req.IndustryLevel,
		IndustryRole:     req.IndustryRole,
		Location:         req.Location,
		LevelRationale:   req.LevelRationale,
		StartedOn:        startedOn,
		EndedOn:          endedOn,
		ContextNarrative: req.ContextNarrative,
		SortOrder:        req.SortOrder,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "position not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update position")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, position)
}

func (h *PositionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "position id must be a valid UUID")
		return
	}

	count, err := h.queries.CountContributionsByPosition(r.Context(), db.CountContributionsByPositionParams{
		PositionID: id,
		UserID:     userID,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to check position dependents")
		return
	}
	if count > 0 {
		httputil.WriteError(w, http.StatusConflict, "has_dependents", "position has contributions and cannot be deleted")
		return
	}

	if err := h.queries.DeletePosition(r.Context(), db.DeletePositionParams{
		ID:     id,
		UserID: userID,
	}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete position")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PositionHandler) ListByEmployer(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	employerID, err := uuid.Parse(chi.URLParam(r, "employerID"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "employer id must be a valid UUID")
		return
	}

	positions, err := h.queries.GetPositionsByEmployer(r.Context(), db.GetPositionsByEmployerParams{
		EmployerID: employerID,
		UserID:     userID,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch positions")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, positions)
}

func (h *PositionHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "position id must be a valid UUID")
		return
	}

	position, err := h.queries.GetPosition(r.Context(), db.GetPositionParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "position not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch position")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, position)
}
