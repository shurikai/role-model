package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/httputil"
)

type ContributionHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewContributionHandler(pool *pgxpool.Pool, queries *db.Queries) *ContributionHandler {
	return &ContributionHandler{pool: pool, queries: queries}
}

type createContributionRequest struct {
	PositionID      uuid.UUID `json:"position_id"`
	Summary         string    `json:"summary"`
	FullDescription string    `json:"full_description"`
	Outcomes        *string   `json:"outcomes"`
	ScaleContext    *string   `json:"scale_context"`
	IsActive        *bool     `json:"is_active"`
}

type updateContributionRequest struct {
	Summary         string  `json:"summary"`
	FullDescription string  `json:"full_description"`
	Outcomes        *string `json:"outcomes"`
	ScaleContext    *string `json:"scale_context"`
	IsActive        *bool   `json:"is_active"`
}

func (h *ContributionHandler) ListByPosition(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	positionID, err := uuid.Parse(chi.URLParam(r, "positionID"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "position id must be a valid UUID")
		return
	}

	contributions, err := h.queries.GetContributionsByPosition(r.Context(), db.GetContributionsByPositionParams{
		PositionID: positionID,
		UserID:     userID,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch contributions")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, contributions)
}

func (h *ContributionHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "contribution id must be a valid UUID")
		return
	}

	contribution, err := h.queries.GetContribution(r.Context(), db.GetContributionParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "contribution not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch contribution")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, contribution)
}

func (h *ContributionHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req createContributionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.PositionID == uuid.Nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "position_id is required")
		return
	}
	if req.Summary == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "summary is required")
		return
	}
	if req.FullDescription == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "full description is required")
		return
	}

	// Parent-ownership check: the position must exist AND belong to this user.
	if _, err := h.queries.GetPosition(r.Context(), db.GetPositionParams{
		ID:     req.PositionID,
		UserID: userID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "position_not_found", "position not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify position")
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	contribution, err := h.queries.CreateContribution(r.Context(), db.CreateContributionParams{
		ID:              uuid.New(),
		UserID:          userID,
		PositionID:      req.PositionID,
		Summary:         req.Summary,
		FullDescription: req.FullDescription,
		Outcomes:        req.Outcomes,
		ScaleContext:    req.ScaleContext,
		IsActive:        isActive,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create contribution")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, contribution)
}

func (h *ContributionHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "contribution id must be a valid UUID")
		return
	}

	var req updateContributionRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Summary == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "summary is required")
		return
	}
	if req.FullDescription == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "full description is required")
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	contribution, err := h.queries.UpdateContribution(r.Context(), db.UpdateContributionParams{
		ID:              id,
		UserID:          userID,
		Summary:         req.Summary,
		FullDescription: req.FullDescription,
		Outcomes:        req.Outcomes,
		ScaleContext:    req.ScaleContext,
		IsActive:        isActive,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "contribution not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update contribution")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, contribution)
}

func (h *ContributionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "contribution id must be a valid UUID")
		return
	}

	// Ownership check before starting the transaction.
	if _, err := h.queries.GetContribution(r.Context(), db.GetContributionParams{
		ID:     id,
		UserID: userID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "contribution not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify contribution")
		return
	}

	// Transaction: remove join rows and the contribution atomically.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to begin transaction")
		return
	}
	defer tx.Rollback(r.Context()) // no-op after a successful commit

	qtx := h.queries.WithTx(tx)

	if err := qtx.DeleteContributionTags(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to remove tag links")
		return
	}
	if err := qtx.DeleteContributionProjectLinks(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to remove project links")
		return
	}
	if err := qtx.DeleteContribution(r.Context(), db.DeleteContributionParams{
		ID:     id,
		UserID: userID,
	}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete contribution")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to commit deletion")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
