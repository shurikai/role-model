package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/httputil"
	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

type GenerationHandler struct {
	queries *db.Queries
	svc     *generation.Service
}

func NewGenerationHandler(queries *db.Queries, svc *generation.Service) *GenerationHandler {
	return &GenerationHandler{queries: queries, svc: svc}
}

func (h *GenerationHandler) ExtractSignals(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "application id must be a valid UUID")
		return
	}

	app, err := h.queries.GetApplication(r.Context(), db.GetApplicationParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "application not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch application")
		return
	}

	if app.JdText == nil || *app.JdText == "" {
		httputil.WriteError(w, http.StatusBadRequest, "no_jd_text", "application has no job description text to analyze")
		return
	}

	signals, err := h.svc.ExtractSignals(r.Context(), *app.JdText)
	if err != nil {
		log.Printf("extract signals: %v", err)
		httputil.WriteError(w, http.StatusBadGateway, "extraction_failed", "failed to extract signals from job description")
		return
	}

	signalsJSON, err := json.Marshal(signals)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to encode signals")
		return
	}

	raw := json.RawMessage(signalsJSON)
	updated, err := h.queries.UpdateApplicationSignals(r.Context(), db.UpdateApplicationSignalsParams{
		ID:        id,
		UserID:    userID,
		JdSignals: &raw,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to store signals")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, updated)
}

func (h *GenerationHandler) Generate(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "application id must be a valid UUID")
		return
	}

	rv, err := h.svc.Generate(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, generation.ErrSignalsRequired) {
			httputil.WriteError(w, http.StatusConflict, "signals_required", "jd_signals must be extracted before generating a resume")
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "application not found")
			return
		}
		log.Printf("generate resume: %v", err)
		if isValidationError(err) {
			httputil.WriteError(w, http.StatusBadGateway, "invalid_generation", "generated resume failed schema validation")
			return
		}
		httputil.WriteError(w, http.StatusBadGateway, "generation_failed", "failed to generate resume")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, rv)
}

// isValidationError reports whether err came from JSON schema validation.
func isValidationError(err error) bool {
	var ve *generation.ValidationError
	return errors.As(err, &ve)
}
