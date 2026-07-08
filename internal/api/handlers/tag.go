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

type TagHandler struct {
	queries *db.Queries
}

func NewTagHandler(queries *db.Queries) *TagHandler {
	return &TagHandler{queries: queries}
}

type tagCategoryRequest struct {
	Name      string `json:"name"`
	SortOrder *int32 `json:"sort_order"`
}

type tagRequest struct {
	Name      string   `json:"name"`
	Category  string   `json:"category"`
	Aliases   []string `json:"aliases"`
	SortOrder *int32   `json:"sort_order"`
}

func (h *TagHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req tagCategoryRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}

	sortOrder := int32(0)
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}

	category, err := h.queries.CreateTagCategory(r.Context(), db.CreateTagCategoryParams{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      req.Name,
		SortOrder: sortOrder,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create tag category")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, category)
}

func (h *TagHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	categories, err := h.queries.ListTagCategories(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch tag categories")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, categories)
}

func (h *TagHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "tag category id must be a valid UUID")
		return
	}

	// Categories are referenced by name (not id) from tags, so resolve the
	// name first to run the dependents guard.
	categories, err := h.queries.ListTagCategories(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify tag category")
		return
	}
	var name string
	found := false
	for _, c := range categories {
		if c.ID == id {
			name = c.Name
			found = true
			break
		}
	}
	if !found {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "tag category not found")
		return
	}

	count, err := h.queries.CountTagsInCategory(r.Context(), db.CountTagsInCategoryParams{
		UserID:   userID,
		Category: name,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to check tag category usage")
		return
	}
	if count > 0 {
		httputil.WriteError(w, http.StatusConflict, "has_dependents", "category has tags and cannot be deleted")
		return
	}

	rows, err := h.queries.DeleteTagCategory(r.Context(), db.DeleteTagCategoryParams{ID: id, UserID: userID})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete tag category")
		return
	}
	if rows == 0 {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "tag category not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TagHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req tagRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	if req.Category == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "category is required")
		return
	}

	// The category must exist for this user (composite FK on tags.category).
	categories, err := h.queries.ListTagCategories(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify category")
		return
	}
	categoryExists := false
	for _, c := range categories {
		if c.Name == req.Category {
			categoryExists = true
			break
		}
	}
	if !categoryExists {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_category", "category does not exist")
		return
	}

	sortOrder := int32(0)
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}

	tag, err := h.queries.CreateTag(r.Context(), db.CreateTagParams{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      req.Name,
		Aliases:   req.Aliases,
		Category:  req.Category,
		SortOrder: sortOrder,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create tag")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, tag)
}

func (h *TagHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	tags, err := h.queries.ListTags(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch tags")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, tags)
}

func (h *TagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "tag id must be a valid UUID")
		return
	}

	count, err := h.queries.CountTagUsage(r.Context(), id)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to check tag usage")
		return
	}
	if count > 0 {
		httputil.WriteError(w, http.StatusConflict, "has_dependents", "tag is assigned and cannot be deleted")
		return
	}

	rows, err := h.queries.DeleteTag(r.Context(), db.DeleteTagParams{ID: id, UserID: userID})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete tag")
		return
	}
	if rows == 0 {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "tag not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TagHandler) AssignToContribution(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	contribID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "contribution id must be a valid UUID")
		return
	}

	var req struct {
		TagID uuid.UUID `json:"tag_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	// Double-ownership: BOTH the contribution and the tag must belong to this user.
	if _, err := h.queries.GetContribution(r.Context(), db.GetContributionParams{ID: contribID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "contribution not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify contribution")
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

	if err := h.queries.AssignTagToContribution(r.Context(), db.AssignTagToContributionParams{
		ContributionID: contribID,
		TagID:          req.TagID,
	}); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to assign tag")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TagHandler) UnassignFromContribution(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	contribID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "contribution id must be a valid UUID")
		return
	}
	tagID, err := uuid.Parse(chi.URLParam(r, "tagId"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "tag id must be a valid UUID")
		return
	}

	// The contribution must belong to this user before its tag assignments can be touched.
	if _, err := h.queries.GetContribution(r.Context(), db.GetContributionParams{ID: contribID, UserID: userID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "contribution not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to verify contribution")
		return
	}

	rows, err := h.queries.UnassignTagFromContribution(r.Context(), db.UnassignTagFromContributionParams{
		ContributionID: contribID,
		TagID:          tagID,
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
