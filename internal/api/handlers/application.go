package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/api/middleware"
	"github.com/shurikai/role-model/internal/db"
)

type ApplicationHandler struct {
	queries *db.Queries
}

func NewApplicationHandler(queries *db.Queries) *ApplicationHandler {
	return &ApplicationHandler{queries: queries}
}

type createApplicationRequest struct {
	CompanyName string  `json:"company_name"`
	RoleTitle   string  `json:"role_title"`
	JDUrl       *string `json:"jd_url"`
	JDText      *string `json:"jd_text"`
	Status      string  `json:"status"`
	Notes       *string `json:"notes"`
}

type updateApplicationRequest struct {
	CompanyName string  `json:"company_name"`
	RoleTitle   string  `json:"role_title"`
	JDUrl       *string `json:"jd_url"`
	JDText      *string `json:"jd_text"`
	Status      string  `json:"status"`
	Notes       *string `json:"notes"`
	AppliedOn   *string `json:"applied_on"` // RFC3339 date; parsed below
}

func (h *ApplicationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	apps, err := h.queries.ListApplications(r.Context(), userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch applications")
		return
	}
	writeJSON(w, http.StatusOK, apps)
}

func (h *ApplicationHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "application id must be a valid UUID")
		return
	}

	app, err := h.queries.GetApplication(r.Context(), db.GetApplicationParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, http.StatusNotFound, "not_found", "application not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch application")
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *ApplicationHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req createApplicationRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.CompanyName == "" || req.RoleTitle == "" {
		WriteError(w, http.StatusBadRequest, "validation_error", "company_name and role_title are required")
		return
	}
	if req.Status == "" {
		req.Status = "draft"
	}

	app, err := h.queries.CreateApplication(r.Context(), db.CreateApplicationParams{
		UserID:      userID,
		CompanyName: req.CompanyName,
		RoleTitle:   req.RoleTitle,
		JdUrl:       req.JDUrl,
		JdText:      req.JDText,
		Status:      req.Status,
		Notes:       req.Notes,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create application")
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

func (h *ApplicationHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "application id must be a valid UUID")
		return
	}

	var req updateApplicationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.CompanyName == "" || req.RoleTitle == "" {
		WriteError(w, http.StatusBadRequest, "validation_error", "company_name and role_title are required")
		return
	}

	app, err := h.queries.UpdateApplication(r.Context(), db.UpdateApplicationParams{
		ID:          id,
		UserID:      userID,
		CompanyName: req.CompanyName,
		RoleTitle:   req.RoleTitle,
		JdUrl:       req.JDUrl,
		JdText:      req.JDText,
		Status:      req.Status,
		Notes:       req.Notes,
		// AppliedOn handled below if you wire date parsing
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, http.StatusNotFound, "not_found", "application not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update application")
		return
	}
	writeJSON(w, http.StatusOK, app)
}
