package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/fitgate"
	"github.com/shurikai/role-model/internal/httputil"
)

type FitHandler struct {
	queries *db.Queries
	svc     *fitgate.Service
}

func NewFitHandler(queries *db.Queries, svc *fitgate.Service) *FitHandler {
	return &FitHandler{queries: queries, svc: svc}
}

func (h *FitHandler) Run(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	appID, err := uuid.Parse(chi.URLParam(r, "applicationID"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "application id must be a valid UUID")
		return
	}

	report, err := h.svc.RunFitEvaluation(r.Context(), userID, appID)
	if err != nil {
		if errors.Is(err, fitgate.ErrSignalsRequired) {
			httputil.WriteError(w, http.StatusUnprocessableEntity, "signals_missing", "Stage 1 extraction required before fit evaluation")
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "application not found")
			return
		}
		log.Printf("run fit evaluation: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "fit_evaluation_failed", "failed to run fit evaluation")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, report)
}

func (h *FitHandler) ListByApplication(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	appID, err := uuid.Parse(chi.URLParam(r, "applicationID"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "application id must be a valid UUID")
		return
	}

	reports, err := h.queries.ListFitReportsByApplication(r.Context(), db.ListFitReportsByApplicationParams{
		ApplicationID: pgtype.UUID{Bytes: [16]byte(appID), Valid: true},
		UserID:        userID,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch fit reports")
		return
	}
	if reports == nil {
		reports = []db.FitReport{}
	}

	httputil.WriteJSON(w, http.StatusOK, reports)
}

func (h *FitHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	reportID, err := uuid.Parse(chi.URLParam(r, "reportID"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "fit report id must be a valid UUID")
		return
	}

	report, err := h.queries.GetFitReport(r.Context(), db.GetFitReportParams{
		ID:     reportID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "fit report not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch fit report")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, report)
}
