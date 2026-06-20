package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

type GenerationHandler struct {
	queries *db.Queries
	client  *generation.Client
}

func NewGenerationHandler(queries *db.Queries, client *generation.Client) *GenerationHandler {
	return &GenerationHandler{queries: queries, client: client}
}

func (h *GenerationHandler) ExtractSignals(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "application id must be a valid UUID")
		return
	}

	app, err := h.queries.GetApplication(r.Context(), db.GetApplicationParams{
		ID:     id,
		UserID: stubUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, http.StatusNotFound, "not_found", "application not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch application")
		return
	}

	if app.JdText == nil || *app.JdText == "" {
		WriteError(w, http.StatusBadRequest, "no_jd_text", "application has no job description text to analyze")
		return
	}

	signals, err := h.client.ExtractSignals(r.Context(), *app.JdText)
	if err != nil {
		log.Printf("extract signals: %v", err)
		WriteError(w, http.StatusBadGateway, "extraction_failed", "failed to extract signals from job description")
		return
	}

	signalsJSON, err := json.Marshal(signals)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to encode signals")
		return
	}

	raw := json.RawMessage(signalsJSON)
	updated, err := h.queries.UpdateApplicationSignals(r.Context(), db.UpdateApplicationSignalsParams{
		ID:        id,
		UserID:    stubUserID,
		JdSignals: &raw,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to store signals")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}
