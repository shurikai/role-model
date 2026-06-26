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

type ResumeVersionHandler struct {
	queries *db.Queries
}

func NewResumeVersionHandler(queries *db.Queries) *ResumeVersionHandler {
	return &ResumeVersionHandler{queries: queries}
}

func (h *ResumeVersionHandler) ListByApplication(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "application id must be a valid UUID")
		return
	}

	versions, err := h.queries.ListResumeVersions(r.Context(), db.ListResumeVersionsParams{
		ApplicationID: appID,
		UserID:        userID,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch resume versions")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, versions)
}

func (h *ResumeVersionHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "resume version id must be a valid UUID")
		return
	}

	version, err := h.queries.GetResumeVersion(r.Context(), db.GetResumeVersionParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "resume version not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch resume version")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, version)
}
