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

type CredentialHandler struct {
	queries *db.Queries
}

func NewCredentialHandler(queries *db.Queries) *CredentialHandler {
	return &CredentialHandler{queries: queries}
}

type credentialRequest struct {
	Name          string  `json:"name"`
	Issuer        *string `json:"issuer"`
	IssuedOn      *string `json:"issued_on"`  // nullable date
	ExpiresOn     *string `json:"expires_on"` // nullable date
	CredentialUrl *string `json:"credential_url"`
}

func (h *CredentialHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	credentials, err := h.queries.GetCredentials(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch credentials")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, credentials)
}

func (h *CredentialHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req credentialRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "credential name is required")
		return
	}

	issuedOn, err := httputil.ParseNullableDate(req.IssuedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "issued_on must be YYYY-MM-DD or null")
		return
	}
	expiresOn, err := httputil.ParseNullableDate(req.ExpiresOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "expires_on must be YYYY-MM-DD or null")
		return
	}

	edu, err := h.queries.CreateCredential(r.Context(), db.CreateCredentialParams{
		ID:            uuid.New(),
		UserID:        userID,
		Name:          req.Name,
		Issuer:        req.Issuer,
		IssuedOn:      issuedOn,
		ExpiresOn:     expiresOn,
		CredentialUrl: req.CredentialUrl,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create credential")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, edu)
}

func (h *CredentialHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "credential id must be a valid UUID")
		return
	}

	var req credentialRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}

	issuedOn, err := httputil.ParseNullableDate(req.IssuedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "started_on must be YYYY-MM-DD or null")
		return
	}
	expiresOn, err := httputil.ParseNullableDate(req.ExpiresOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "ended_on must be YYYY-MM-DD or null")
		return
	}

	edu, err := h.queries.UpdateCredential(r.Context(), db.UpdateCredentialParams{
		ID:            id,
		UserID:        userID,
		Name:          req.Name,
		Issuer:        req.Issuer,
		IssuedOn:      issuedOn,
		ExpiresOn:     expiresOn,
		CredentialUrl: req.CredentialUrl,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "credential not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update credential")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, edu)
}

func (h *CredentialHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "credential id must be a valid UUID")
		return
	}

	rows, err := h.queries.DeleteCredential(r.Context(), db.DeleteCredentialParams{ID: id, UserID: userID})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete credential")
		return
	}
	if rows == 0 {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "credential not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
