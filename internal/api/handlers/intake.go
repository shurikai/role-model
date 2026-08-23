package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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
}

func NewIntakeHandler(queries *db.Queries, svc *intake.Service) *IntakeHandler {
	return &IntakeHandler{queries: queries, svc: svc}
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
