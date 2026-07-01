package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/httputil"
	"github.com/shurikai/role-model/internal/stage0"
)

type ImportHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	stage0  *stage0.Service
}

func NewImportHandler(pool *pgxpool.Pool, queries *db.Queries, stage0Svc *stage0.Service) *ImportHandler {
	return &ImportHandler{pool: pool, queries: queries, stage0: stage0Svc}
}

type createImportBatchRequest struct {
	RawText string `json:"raw_text"`
}

type createImportBatchResponse struct {
	ID         uuid.UUID `json:"id"`
	Status     string    `json:"status"`
	ErrorText  *string   `json:"error_text,omitempty"`
	DraftCount int64     `json:"draft_count"`
}

type importBatchWithCounts struct {
	db.ImportBatch
	DraftCounts draftCounts `json:"draft_counts"`
}

type draftCounts struct {
	Total    int64 `json:"total"`
	Pending  int64 `json:"pending"`
	Approved int64 `json:"approved"`
	Rejected int64 `json:"rejected"`
}

// Create starts a new import batch and runs Stage 0 extraction + enrichment
// synchronously before responding.
func (h *ImportHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req createImportBatchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.RawText) == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "raw_text is required")
		return
	}

	batchID := uuid.New()
	if _, err := h.queries.CreateImportBatch(r.Context(), db.CreateImportBatchParams{
		ID:      batchID,
		UserID:  userID,
		RawText: req.RawText,
		Status:  "pending",
	}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create import batch")
		return
	}

	if err := h.stage0.RunExtraction(r.Context(), batchID, userID); err != nil {
		log.Printf("stage0: extraction failed for batch %s: %v", batchID, err)
	}

	batch, err := h.queries.GetImportBatch(r.Context(), db.GetImportBatchParams{ID: batchID, UserID: userID})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch import batch")
		return
	}

	counts, err := h.queries.CountContributionDraftsByBatch(r.Context(), db.CountContributionDraftsByBatchParams{
		BatchID: batchID,
		UserID:  userID,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to count drafts")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, createImportBatchResponse{
		ID:         batch.ID,
		Status:     batch.Status,
		ErrorText:  batch.ErrorText,
		DraftCount: counts.Total,
	})
}

// GetBatch returns the batch row plus draft status counts.
func (h *ImportHandler) GetBatch(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	batchID, err := uuid.Parse(chi.URLParam(r, "batchID"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "batch id must be a valid UUID")
		return
	}

	batch, err := h.queries.GetImportBatch(r.Context(), db.GetImportBatchParams{ID: batchID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "import batch not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch import batch")
		return
	}

	counts, err := h.queries.CountContributionDraftsByBatch(r.Context(), db.CountContributionDraftsByBatchParams{
		BatchID: batchID,
		UserID:  userID,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to count drafts")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, importBatchWithCounts{
		ImportBatch: batch,
		DraftCounts: draftCounts{
			Total:    counts.Total,
			Pending:  counts.Pending,
			Approved: counts.Approved,
			Rejected: counts.Rejected,
		},
	})
}

// ListDrafts returns all drafts for a batch, after verifying the batch
// belongs to the authenticated user.
func (h *ImportHandler) ListDrafts(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	batchID, err := uuid.Parse(chi.URLParam(r, "batchID"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "batch id must be a valid UUID")
		return
	}

	if _, err := h.queries.GetImportBatch(r.Context(), db.GetImportBatchParams{ID: batchID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "import batch not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify import batch")
		return
	}

	drafts, err := h.queries.ListContributionDraftsByBatch(r.Context(), db.ListContributionDraftsByBatchParams{
		BatchID: batchID,
		UserID:  userID,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch drafts")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, drafts)
}

// GetDraft returns a single draft, user-scoped.
func (h *ImportHandler) GetDraft(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "draftID"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "draft id must be a valid UUID")
		return
	}

	draft, err := h.queries.GetContributionDraft(r.Context(), db.GetContributionDraftParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "draft not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch draft")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, draft)
}

var updatableDraftFields = map[string]bool{
	"summary":          true,
	"full_description": true,
	"outcomes":         true,
	"scale_context":    true,
}

// UpdateDraft applies a partial update — only fields present in the request
// body are changed; omitted fields keep their existing value.
func (h *ImportHandler) UpdateDraft(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "draftID"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "draft id must be a valid UUID")
		return
	}

	existing, err := h.queries.GetContributionDraft(r.Context(), db.GetContributionDraftParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "draft not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch draft")
		return
	}

	var raw map[string]json.RawMessage
	if !decodeJSON(w, r, &raw) {
		return
	}

	summary := existing.Summary
	fullDescription := existing.FullDescription
	outcomes := existing.Outcomes
	scaleContext := existing.ScaleContext

	for key, value := range raw {
		if !updatableDraftFields[key] {
			httputil.WriteError(w, http.StatusBadRequest, "invalid_field", fmt.Sprintf("unknown or non-editable field %q", key))
			return
		}
		var s *string
		if err := json.Unmarshal(value, &s); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid_body", fmt.Sprintf("field %q must be a string or null", key))
			return
		}
		switch key {
		case "summary":
			summary = s
		case "full_description":
			fullDescription = s
		case "outcomes":
			outcomes = s
		case "scale_context":
			scaleContext = s
		}
	}

	draft, err := h.queries.UpdateContributionDraft(r.Context(), db.UpdateContributionDraftParams{
		ID:              id,
		UserID:          userID,
		Summary:         summary,
		FullDescription: fullDescription,
		Outcomes:        outcomes,
		ScaleContext:    scaleContext,
		Flags:           existing.Flags,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "draft not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update draft")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, draft)
}

type approveDraftRequest struct {
	PositionID uuid.UUID `json:"position_id"`
}

// Approve verifies the target position belongs to the user, writes a new
// contribution from the draft's fields, and marks the draft approved.
func (h *ImportHandler) Approve(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "draftID"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "draft id must be a valid UUID")
		return
	}

	var req approveDraftRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.PositionID == uuid.Nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "position_id is required")
		return
	}

	draft, err := h.queries.GetContributionDraft(r.Context(), db.GetContributionDraftParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "draft not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch draft")
		return
	}
	if draft.Status != "pending" {
		httputil.WriteError(w, http.StatusConflict, "invalid_state", "draft is not pending")
		return
	}
	if draft.Summary == nil || *draft.Summary == "" {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "validation_error", "draft summary must be set before approval")
		return
	}
	if draft.FullDescription == nil || *draft.FullDescription == "" {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "validation_error", "draft full_description must be set before approval")
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

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to begin transaction")
		return
	}
	defer tx.Rollback(r.Context()) // no-op after a successful commit

	qtx := h.queries.WithTx(tx)

	contribution, err := qtx.CreateContribution(r.Context(), db.CreateContributionParams{
		ID:              uuid.New(),
		UserID:          userID,
		PositionID:      req.PositionID,
		Summary:         *draft.Summary,
		FullDescription: *draft.FullDescription,
		Outcomes:        draft.Outcomes,
		ScaleContext:    draft.ScaleContext,
		IsActive:        true,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create contribution")
		return
	}

	if _, err := qtx.UpdateContributionDraftStatus(r.Context(), db.UpdateContributionDraftStatusParams{
		ID:     id,
		UserID: userID,
		Status: "approved",
	}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update draft status")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to commit approval")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]uuid.UUID{"contribution_id": contribution.ID})
}

// Reject marks a draft rejected. No body required.
func (h *ImportHandler) Reject(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "draftID"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "draft id must be a valid UUID")
		return
	}

	draft, err := h.queries.UpdateContributionDraftStatus(r.Context(), db.UpdateContributionDraftStatusParams{
		ID:     id,
		UserID: userID,
		Status: "rejected",
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "draft not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to reject draft")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"id":     draft.ID.String(),
		"status": draft.Status,
	})
}
