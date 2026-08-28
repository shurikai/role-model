package handlers

import (
	"net/http"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/httputil"
)

// VocabularyHandler exposes the user-owned vocabularies a client needs in
// order to construct a valid row.
//
// Read-only, and deliberately so. These tables are installed at signup and
// edited rarely; what a client cannot do without is *knowing what is in
// them*. The skills form is the motivating case: proficiency is validated
// against the account's own proficiency_levels rows rather than a hardcoded
// scale, so a form that guessed novice/proficient/expert would be guessing at
// a vocabulary that is per-account by design — right for a new account today,
// wrong for anyone who has edited theirs.
//
// Editing them is a separate decision. A screen for renaming a rung has to
// answer what happens to the positions already filed under the old name, and
// that question deserves more than a PATCH route.
type VocabularyHandler struct {
	queries *db.Queries
}

func NewVocabularyHandler(queries *db.Queries) *VocabularyHandler {
	return &VocabularyHandler{queries: queries}
}

// ListProficiencyLevels returns the account's depth scale, in sort order.
func (h *VocabularyHandler) ListProficiencyLevels(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	levels, err := h.queries.ListProficiencyLevelsByUser(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch proficiency levels")
		return
	}
	// An account with no rows gets an empty array rather than null, so a client
	// can render "no scale configured" instead of crashing on a nil map. The
	// server accepts any proficiency in that state — see SkillHandler.
	if levels == nil {
		levels = []db.ProficiencyLevel{}
	}
	httputil.WriteJSON(w, http.StatusOK, levels)
}

// ListCareerLevels returns the account's seniority ladder, in sort order.
//
// Not needed by the skills form, but it is the other half of the same
// question — a position's industry_level is filed against these — and
// returning one without the other invites a second round trip later.
func (h *VocabularyHandler) ListCareerLevels(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	levels, err := h.queries.ListCareerLevelsByUser(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch career levels")
		return
	}
	if levels == nil {
		levels = []db.CareerLevel{}
	}
	httputil.WriteJSON(w, http.StatusOK, levels)
}
