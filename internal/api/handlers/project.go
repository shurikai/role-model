package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/httputil"
	"github.com/shurikai/role-model/internal/project"
)

var validProjectRoles = map[string]bool{
	"author":      true,
	"maintainer":  true,
	"contributor": true,
	"lead":        true,
}

var validProjectStatuses = map[string]bool{
	"active":   true,
	"dormant":  true,
	"archived": true,
}

type ProjectHandler struct {
	queries    *db.Queries
	projectSvc *project.Service
}

func NewProjectHandler(queries *db.Queries, projectSvc *project.Service) *ProjectHandler {
	return &ProjectHandler{queries: queries, projectSvc: projectSvc}
}

type projectRequest struct {
	Name         string  `json:"name"`
	Tagline      *string `json:"tagline"`
	Role         string  `json:"role"`
	Status       string  `json:"status"`
	StartedOn    *string `json:"started_on"` // nullable date
	EndedOn      *string `json:"ended_on"`   // nullable date
	RepoURL      *string `json:"repo_url"`
	LiveURL      *string `json:"live_url"`
	WriteupURL   *string `json:"writeup_url"`
	ForceInclude bool    `json:"force_include"`
	ForceExclude bool    `json:"force_exclude"`
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	projects, err := h.queries.GetProjects(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch projects")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, projects)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "project id must be a valid UUID")
		return
	}

	proj, err := h.queries.GetProject(r.Context(), db.GetProjectParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch project")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, proj)
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req projectRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	if !validProjectRoles[req.Role] {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "role must be one of author, maintainer, contributor, lead")
		return
	}
	if !validProjectStatuses[req.Status] {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "status must be one of active, dormant, archived")
		return
	}

	startedOn, err := httputil.ParseNullableDate(req.StartedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "started_on must be YYYY-MM-DD or null")
		return
	}
	endedOn, err := httputil.ParseNullableDate(req.EndedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "ended_on must be YYYY-MM-DD or null")
		return
	}

	proj, err := h.queries.CreateProject(r.Context(), db.CreateProjectParams{
		ID:           uuid.New(),
		UserID:       userID,
		Name:         req.Name,
		Tagline:      req.Tagline,
		Role:         req.Role,
		Status:       req.Status,
		StartedOn:    startedOn,
		EndedOn:      endedOn,
		RepoUrl:      req.RepoURL,
		LiveUrl:      req.LiveURL,
		WriteupUrl:   req.WriteupURL,
		ForceInclude: req.ForceInclude,
		ForceExclude: req.ForceExclude,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create project")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, proj)
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "project id must be a valid UUID")
		return
	}

	var req projectRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	if !validProjectRoles[req.Role] {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "role must be one of author, maintainer, contributor, lead")
		return
	}
	if !validProjectStatuses[req.Status] {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "status must be one of active, dormant, archived")
		return
	}

	startedOn, err := httputil.ParseNullableDate(req.StartedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "started_on must be YYYY-MM-DD or null")
		return
	}
	endedOn, err := httputil.ParseNullableDate(req.EndedOn)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "ended_on must be YYYY-MM-DD or null")
		return
	}

	proj, err := h.queries.UpdateProject(r.Context(), db.UpdateProjectParams{
		ID:           id,
		UserID:       userID,
		Name:         req.Name,
		Tagline:      req.Tagline,
		Role:         req.Role,
		Status:       req.Status,
		StartedOn:    startedOn,
		EndedOn:      endedOn,
		RepoUrl:      req.RepoURL,
		LiveUrl:      req.LiveURL,
		WriteupUrl:   req.WriteupURL,
		ForceInclude: req.ForceInclude,
		ForceExclude: req.ForceExclude,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update project")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, proj)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "project id must be a valid UUID")
		return
	}

	if err := h.projectSvc.Delete(r.Context(), userID, id); err != nil {
		if errors.Is(err, project.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete project")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) AssignContribution(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "project id must be a valid UUID")
		return
	}

	var req struct {
		ContributionID uuid.UUID `json:"contribution_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	// Double-ownership: BOTH the project and the contribution must belong to this user.
	if _, err := h.queries.GetProject(r.Context(), db.GetProjectParams{ID: projectID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify project")
		return
	}
	if _, err := h.queries.GetContribution(r.Context(), db.GetContributionParams{ID: req.ContributionID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "contribution not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify contribution")
		return
	}

	if err := h.queries.AssignContributionToProject(r.Context(), db.AssignContributionToProjectParams{
		ProjectID:      projectID,
		ContributionID: req.ContributionID,
	}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to assign contribution")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) UnassignContribution(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "project id must be a valid UUID")
		return
	}
	contribID, err := uuid.Parse(chi.URLParam(r, "contribID"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "contribution id must be a valid UUID")
		return
	}

	// The project must belong to this user before its contribution links can be touched.
	if _, err := h.queries.GetProject(r.Context(), db.GetProjectParams{ID: projectID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify project")
		return
	}

	rows, err := h.queries.UnassignContributionFromProject(r.Context(), db.UnassignContributionFromProjectParams{
		ProjectID:      projectID,
		ContributionID: contribID,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to unassign contribution")
		return
	}
	if rows == 0 {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "contribution assignment not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) AssignTag(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "project id must be a valid UUID")
		return
	}

	var req struct {
		TagID uuid.UUID `json:"tag_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	// Double-ownership: BOTH the project and the tag must belong to this user.
	if _, err := h.queries.GetProject(r.Context(), db.GetProjectParams{ID: projectID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify project")
		return
	}
	if _, err := h.queries.GetTag(r.Context(), db.GetTagParams{ID: req.TagID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "tag not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify tag")
		return
	}

	if err := h.queries.AssignTagToProject(r.Context(), db.AssignTagToProjectParams{
		ProjectID: projectID,
		TagID:     req.TagID,
	}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to assign tag")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) UnassignTag(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "project id must be a valid UUID")
		return
	}
	tagID, err := uuid.Parse(chi.URLParam(r, "tagId"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "tag id must be a valid UUID")
		return
	}

	// The project must belong to this user before its tag links can be touched.
	if _, err := h.queries.GetProject(r.Context(), db.GetProjectParams{ID: projectID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify project")
		return
	}

	rows, err := h.queries.UnassignTagFromProject(r.Context(), db.UnassignTagFromProjectParams{
		ProjectID: projectID,
		TagID:     tagID,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to unassign tag")
		return
	}
	if rows == 0 {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "tag assignment not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
