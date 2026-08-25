package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/httputil"
	"github.com/shurikai/role-model/internal/intake"
)

// IntakeHandler exposes the widened draft surface: everything a career is made
// of, not contributions alone.
//
// The contribution-only endpoints on ImportHandler stay. They are the narrower
// path — approving one drafted contribution against a position that already
// exists — and it is still the right shape once an account has a career in it.
// These endpoints are for the case that had no path at all: a new account, and
// a batch that has to create the employers and positions before anything can
// hang off them.
type IntakeHandler struct {
	queries *db.Queries
	svc     *intake.Service
	// The model half of career extraction. Nil is a legal state — a server
	// with no API key configured serves every other intake endpoint and
	// refuses only the one that spends money, which is better than failing to
	// start or panicking on the first import.
	extractor intake.Extractor
}

func NewIntakeHandler(queries *db.Queries, svc *intake.Service, extractor intake.Extractor) *IntakeHandler {
	return &IntakeHandler{queries: queries, svc: svc, extractor: extractor}
}

type entityDraftDTO struct {
	ID         uuid.UUID   `json:"id"`
	BatchID    uuid.UUID   `json:"batch_id"`
	Kind       string      `json:"kind"`
	Payload    any         `json:"payload"`
	DependsOn  []uuid.UUID `json:"depends_on"`
	ResolvedID *uuid.UUID  `json:"resolved_id"`
	Flags      any         `json:"flags"`
	Status     string      `json:"status"`
}

// ListEntityDrafts returns every drafted entity in a batch, resolved ones
// included — a reviewer needs to see what a previous pass already created, or
// the queue looks like it lost rows.
func (h *IntakeHandler) ListEntityDrafts(w http.ResponseWriter, r *http.Request) {
	userID, batchID, ok := h.scope(w, r, "batchID")
	if !ok {
		return
	}

	drafts, err := h.queries.ListEntityDraftsByBatch(r.Context(), db.ListEntityDraftsByBatchParams{
		BatchID: batchID,
		UserID:  userID,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list drafts")
		return
	}

	out := make([]entityDraftDTO, 0, len(drafts))
	for _, d := range drafts {
		out = append(out, entityDraftDTO{
			ID: d.ID, BatchID: d.BatchID, Kind: d.Kind,
			Payload: d.Payload, DependsOn: d.DependsOn,
			ResolvedID: d.ResolvedID, Flags: d.Flags, Status: d.Status,
		})
	}
	httputil.WriteJSON(w, http.StatusOK, out)
}

type resolveBatchResponse struct {
	Resolved   map[string]string `json:"resolved"`
	Unresolved map[string]string `json:"unresolved"`
}

// ResolveBatch writes every pending draft in the batch, in dependency order,
// inside one transaction.
//
// A batch that resolves partially returns 409 with the reasons rather than 200
// with a quiet subset: "eleven of your fourteen drafts became rows and three
// did not" is a thing the caller has to be told, not something to discover by
// counting.
func (h *IntakeHandler) ResolveBatch(w http.ResponseWriter, r *http.Request) {
	userID, batchID, ok := h.scope(w, r, "batchID")
	if !ok {
		return
	}

	result, err := h.svc.ResolveBatch(r.Context(), userID, batchID)
	body := resolveBatchResponse{
		Resolved:   map[string]string{},
		Unresolved: map[string]string{},
	}
	for draftID, rowID := range result.Resolved {
		body.Resolved[draftID.String()] = rowID.String()
	}
	for draftID, why := range result.Unresolved {
		body.Unresolved[draftID.String()] = why
	}

	switch {
	case err == nil:
		httputil.WriteJSON(w, http.StatusOK, body)
	case errors.Is(err, intake.ErrUnresolved):
		httputil.WriteJSON(w, http.StatusConflict, body)
	default:
		log.Printf("resolve batch %s: %v", batchID, err)
		httputil.WriteError(w, http.StatusInternalServerError, "resolve_failed",
			"failed to resolve the batch; nothing was written")
	}
}

func (h *IntakeHandler) scope(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", param+" must be a valid UUID")
		return uuid.Nil, uuid.Nil, false
	}
	return userID, id, true
}

type startCareerImportRequest struct {
	RawText string `json:"raw_text"`
}

type startCareerImportResponse struct {
	ID         uuid.UUID      `json:"id"`
	Status     string         `json:"status"`
	ErrorText  *string        `json:"error_text,omitempty"`
	DraftCount int            `json:"draft_count"`
	ByKind     map[string]int `json:"draft_count_by_kind"`
}

// StartCareerImport is the whole-career path's front door: pasted text in,
// staged drafts out, in one request.
//
// Until now this existed only in cmd/intakerun, which also creates the account
// it imports into. That step is deliberately not repeated here — a request
// carrying an authenticated user already has one, and signup installs the
// starting vocabularies in the same transaction as the user row
// (AuthHandler.Signup), so the account this runs against is in exactly the
// state intakerun builds by hand.
//
// Kept separate from ImportHandler.Create rather than unified with it. They
// stage different things — contribution_drafts against positions that already
// exist, versus a whole career including the employers and positions — and
// merging them is a larger refactor than either caller needs.
func (h *IntakeHandler) StartCareerImport(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req startCareerImportRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.RawText) == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "raw_text is required")
		return
	}
	if h.extractor == nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, "extraction_unavailable",
			"career extraction is not configured on this server")
		return
	}

	batch, err := h.queries.CreateImportBatch(r.Context(), db.CreateImportBatchParams{
		ID: uuid.New(), UserID: userID, RawText: req.RawText, Status: "extracting",
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create import batch")
		return
	}

	drafts, err := h.svc.ExtractCareer(r.Context(), h.extractor, userID, batch.ID, req.RawText)
	if err != nil {
		// The batch row stays, carrying the reason. A failed extraction that
		// deleted its own batch would leave the person with a spinner that
		// stopped and nothing to read.
		log.Printf("start career import %s: %v", batch.ID, err)
		msg := err.Error()
		failed, ferr := h.queries.UpdateImportBatchStatus(r.Context(), db.UpdateImportBatchStatusParams{
			ID: batch.ID, UserID: userID, Status: "failed", ErrorText: &msg,
		})
		if ferr != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "extraction failed")
			return
		}
		httputil.WriteJSON(w, http.StatusBadGateway, startCareerImportResponse{
			ID: failed.ID, Status: failed.Status, ErrorText: failed.ErrorText,
			DraftCount: 0, ByKind: map[string]int{},
		})
		return
	}

	// 'review' is where a batch waits for a person, and it is what the review
	// screen tests for. Leaving it on 'extracting' would show a finished
	// import as still running, forever.
	ready, err := h.queries.UpdateImportBatchStatus(r.Context(), db.UpdateImportBatchStatusParams{
		ID: batch.ID, UserID: userID, Status: "review",
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update batch status")
		return
	}

	byKind := map[string]int{}
	for _, d := range drafts {
		byKind[d.Kind]++
	}
	httputil.WriteJSON(w, http.StatusCreated, startCareerImportResponse{
		ID: ready.ID, Status: ready.Status, ErrorText: ready.ErrorText,
		DraftCount: len(drafts), ByKind: byKind,
	})
}

type approveEntityDraftResponse struct {
	DraftID    uuid.UUID `json:"draft_id"`
	ResolvedID uuid.UUID `json:"resolved_id"`
}

// ApproveEntityDraft resolves one draft into a real row.
//
// A draft whose parent is still pending is refused with 409 naming that
// parent, never resolved by quietly approving the parent too. Both directions
// of a dependency edge are crossed deliberately: this is the mirror of the
// reject side, where a draft with dependents warns about them first.
func (h *IntakeHandler) ApproveEntityDraft(w http.ResponseWriter, r *http.Request) {
	userID, draftID, ok := h.scope(w, r, "draftID")
	if !ok {
		return
	}

	rowID, err := h.svc.ApproveDraft(r.Context(), userID, draftID)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			httputil.WriteError(w, http.StatusNotFound, "not_found", "draft not found")
		case errors.Is(err, intake.ErrDraftNotPending):
			httputil.WriteError(w, http.StatusConflict, "invalid_state", "draft is not pending")
		case errors.Is(err, intake.ErrDependencyNotResolved):
			// The message names which parent, because "approve the parent
			// first" is useless without saying which card that is.
			httputil.WriteError(w, http.StatusConflict, "dependency_not_resolved", err.Error())
		default:
			log.Printf("approve entity draft %s: %v", draftID, err)
			httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to approve draft")
		}
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, approveEntityDraftResponse{
		DraftID: draftID, ResolvedID: rowID,
	})
}

// RejectEntityDraft marks one draft rejected, from pending only.
//
// Rejecting does not cascade, and does not need to: topoOrder already excludes
// rejected drafts and reports each dependent by name as unresolved, so a
// rejected employer surfaces its orphaned positions at the next resolve rather
// than deleting them out from under the reviewer.
func (h *IntakeHandler) RejectEntityDraft(w http.ResponseWriter, r *http.Request) {
	userID, draftID, ok := h.scope(w, r, "draftID")
	if !ok {
		return
	}

	// Read first, so "no such draft" and "not pending" are different answers.
	// The UPDATE's own guard cannot tell them apart — both match zero rows —
	// and 404 vs 409 is the difference between a broken link and a race with
	// another tab.
	draft, err := h.queries.GetEntityDraft(r.Context(), db.GetEntityDraftParams{ID: draftID, UserID: userID})
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

	rejected, err := h.queries.MarkEntityDraftRejected(r.Context(), db.MarkEntityDraftRejectedParams{
		ID: draftID, UserID: userID,
	})
	if err != nil {
		// Zero rows here means the status moved between the read and the
		// write. The guard is what makes that a 409 rather than a reject
		// stamped over a draft that already became a row.
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusConflict, "invalid_state", "draft is no longer pending")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to reject draft")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"id": rejected.ID.String(), "status": rejected.Status,
	})
}

// UpdateEntityDraftPayload replaces a pending draft's payload wholesale.
//
// Full replace rather than a field-level patch, unlike the narrow path's
// UpdateDraft: five kinds have five payload shapes, the editor for a kind
// always submits a complete object, and there is no per-kind allowlist to keep
// in sync as a result.
func (h *IntakeHandler) UpdateEntityDraftPayload(w http.ResponseWriter, r *http.Request) {
	userID, draftID, ok := h.scope(w, r, "draftID")
	if !ok {
		return
	}

	draft, err := h.queries.GetEntityDraft(r.Context(), db.GetEntityDraftParams{ID: draftID, UserID: userID})
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

	var payload json.RawMessage
	if !decodeJSON(w, r, &payload) {
		return
	}
	// Checked against the draft's own kind before it is stored: an edit that
	// cannot become a row must fail here, on the form, not later during
	// resolution where it reads as the resolver being broken.
	if err := intake.ValidatePayload(draft.Kind, payload); err != nil {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "invalid_payload", err.Error())
		return
	}

	updated, err := h.queries.UpdateEntityDraftPayload(r.Context(), db.UpdateEntityDraftPayloadParams{
		ID: draftID, UserID: userID, Payload: &payload,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusConflict, "invalid_state", "draft is no longer pending")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update draft")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, entityDraftDTO{
		ID: updated.ID, BatchID: updated.BatchID, Kind: updated.Kind,
		Payload: updated.Payload, DependsOn: updated.DependsOn,
		ResolvedID: updated.ResolvedID, Flags: updated.Flags, Status: updated.Status,
	})
}
