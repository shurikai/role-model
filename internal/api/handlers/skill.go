package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/httputil"
	"github.com/shurikai/role-model/internal/intake"
)

// SkillHandler is the CRUD surface for claimed skills.
//
// Like preferences, skills had no HTTP surface: the intake resolver and seed
// SQL were the only writers. Unlike preferences, a skill is not free text — it
// keys to a tag, and the tag keys to a category, so creating one is a
// three-link write that has to happen in order.
type SkillHandler struct {
	queries *db.Queries
}

func NewSkillHandler(queries *db.Queries) *SkillHandler {
	return &SkillHandler{queries: queries}
}

type skillCreateRequest struct {
	Category        string   `json:"category"`
	Tag             string   `json:"tag"`
	Proficiency     string   `json:"proficiency"`
	YearsExperience *float64 `json:"years_experience"`
	IsActive        *bool    `json:"is_active"`
}

type skillUpdateRequest struct {
	Proficiency     string   `json:"proficiency"`
	YearsExperience *float64 `json:"years_experience"`
	IsActive        *bool    `json:"is_active"`
}

// checkProficiency validates against the account's OWN proficiency_levels
// rows, not a hardcoded scale. Migration 020 made the depth scale user-owned
// and dropped the CHECK; a handler that reintroduced novice/proficient/expert
// as a Go constant would put the seventh copy of a vocabulary back where the
// neutrality work removed six.
//
// An account with no rows accepts anything. That mirrors how every other
// reader degrades — LevelScale ranks an unknown value below every band rather
// than failing — and it means an account created outside signup can still
// record a skill instead of being locked out by an empty lookup table.
func (h *SkillHandler) checkProficiency(r *http.Request, userID uuid.UUID, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "proficiency is required", nil
	}
	levels, err := h.queries.ListProficiencyLevelsByUser(r.Context(), userID)
	if err != nil {
		return "", err
	}
	if len(levels) == 0 {
		return "", nil
	}
	allowed := make([]string, 0, len(levels))
	for _, l := range levels {
		if strings.EqualFold(l.Value, value) {
			return "", nil
		}
		allowed = append(allowed, l.Value)
	}
	return "proficiency must be one of: " + strings.Join(allowed, ", "), nil
}

// parseYears converts the request's number to the NUMERIC the column holds.
// Nil stays NULL: an unrecorded duration is not evidence of a short one, which
// is why ListActiveSkillProfileByUser sorts NULLs last rather than as zero.
func parseYears(v *float64) (pgtype.Numeric, error) {
	var out pgtype.Numeric
	if v == nil {
		return out, nil
	}
	if *v < 0 {
		return out, errors.New("years_experience cannot be negative")
	}
	if err := out.Scan(fmt.Sprintf("%.1f", *v)); err != nil {
		return out, err
	}
	return out, nil
}

func (h *SkillHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	skills, err := h.queries.ListSkillsWithTagsByUser(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch skills")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, skills)
}

func (h *SkillHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req skillCreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Category) == "" || strings.TrimSpace(req.Tag) == "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "category and tag are both required")
		return
	}
	msg, err := h.checkProficiency(r, userID, req.Proficiency)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to read proficiency levels")
		return
	}
	if msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", msg)
		return
	}
	years, err := parseYears(req.YearsExperience)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "years_experience must be a non-negative number or null")
		return
	}

	// The same resolve-or-create the import uses, not a second copy. It owns
	// the case-insensitive match that stops "postgresql" becoming a second tag
	// beside "PostgreSQL" and splitting the evidence for every requirement
	// either one answers.
	tag, err := intake.ResolveOrCreateTag(r.Context(), h.queries, userID, req.Category, req.Tag)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to resolve tag")
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	skill, err := h.queries.CreateSkill(r.Context(), db.CreateSkillParams{
		ID:              uuid.New(),
		UserID:          userID,
		TagID:           tag.ID,
		Proficiency:     req.Proficiency,
		YearsExperience: years,
		IsActive:        isActive,
	})
	if err != nil {
		// skills is UNIQUE (user_id, tag_id): a person has one depth for a
		// thing, so a second claim on the same tag is an edit of the first
		// rather than a new row. Saying so is more useful than a 500, and
		// more useful than silently upserting over a depth they set earlier.
		if isUniqueViolation(err) {
			httputil.WriteError(w, http.StatusConflict, "already_exists",
				"this skill already exists; edit the existing one instead")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create skill")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, skill)
}

// Update changes depth and activity only. The tag a skill points at is
// deliberately not editable: repointing it turns the row into a claim about a
// different thing while keeping its id, its history and its provenance links,
// which is a new skill rather than an edit. Delete and re-create instead.
func (h *SkillHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "skill id must be a valid UUID")
		return
	}

	var req skillUpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	msg, err := h.checkProficiency(r, userID, req.Proficiency)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to read proficiency levels")
		return
	}
	if msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", msg)
		return
	}
	years, err := parseYears(req.YearsExperience)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "years_experience must be a non-negative number or null")
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	skill, err := h.queries.UpdateSkill(r.Context(), db.UpdateSkillParams{
		ID:              id,
		UserID:          userID,
		Proficiency:     req.Proficiency,
		YearsExperience: years,
		IsActive:        isActive,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "skill not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update skill")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, skill)
}

// Delete removes the row. The tag it pointed at is left alone: tags are
// vocabulary shared with contributions, and deleting one because a skill claim
// was retracted would strip the term off every contribution carrying it.
func (h *SkillHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "skill id must be a valid UUID")
		return
	}

	rows, err := h.queries.DeleteSkill(r.Context(), db.DeleteSkillParams{ID: id, UserID: userID})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete skill")
		return
	}
	if rows == 0 {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "skill not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
