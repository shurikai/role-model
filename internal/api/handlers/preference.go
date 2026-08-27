package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/httputil"
)

// PreferenceHandler is the CRUD surface for the preference profile.
//
// Preferences drive half the fit gate and had no HTTP surface at all until
// now: the intake resolver was the only writer, so a preference could be
// created exactly once, by an import, and never corrected afterwards. The
// profile is also the part of this system that changes most as a job search
// proceeds — a dealbreaker learned from a bad interview is the normal way one
// gets added, and that happens long after any import.
type PreferenceHandler struct {
	queries *db.Queries
}

func NewPreferenceHandler(queries *db.Queries) *PreferenceHandler {
	return &PreferenceHandler{queries: queries}
}

// preferenceTypes and sentiments mirror the CHECK constraints migration 021
// and migration 011 left on the table.
//
// Validated here as well as in the database so a typo comes back as a 400
// naming the field rather than a 500 wrapping a constraint violation. This is
// a closed vocabulary on purpose and is not the kind of enum the neutrality
// work removed: preference_type names which part of a posting a row is
// checked against, which is a property of the matcher rather than of anyone's
// industry.
var preferenceTypes = map[string]bool{
	"domain": true, "role_shape": true, "culture": true,
	"dealbreaker": true, "core_practice": true,
}

var preferenceSentiments = map[string]bool{"positive": true, "negative": true}

type preferenceRequest struct {
	PreferenceType string   `json:"preference_type"`
	Label          string   `json:"label"`
	Aliases        []string `json:"aliases"`
	Sentiment      string   `json:"sentiment"`
	Weight         int16    `json:"weight"`
	IsHardGate     bool     `json:"is_hard_gate"`
	ContextType    *string  `json:"context_type"`
	Notes          *string  `json:"notes"`
}

// validate returns a message for the first problem, or "" when the request is
// usable. Ordered so the most likely typo is reported first.
func (r preferenceRequest) validate() string {
	if strings.TrimSpace(r.Label) == "" {
		return "label is required"
	}
	if !preferenceTypes[r.PreferenceType] {
		return "preference_type must be one of: domain, role_shape, culture, dealbreaker, core_practice"
	}
	if !preferenceSentiments[r.Sentiment] {
		return "sentiment must be positive or negative"
	}
	if r.Weight < 1 || r.Weight > 10 {
		return "weight must be between 1 and 10"
	}
	return ""
}

// cleanAliases drops blanks and trims. An alias that is empty or whitespace
// would match nothing and read as a populated row, which is worse than an
// absent one: the fit gate would report a gap the person believes they closed.
func cleanAliases(in []string) []string {
	out := make([]string, 0, len(in))
	for _, a := range in {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (h *PreferenceHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	preferences, err := h.queries.ListPreferencesByUser(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to fetch preferences")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, preferences)
}

func (h *PreferenceHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req preferenceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", msg)
		return
	}

	pref, err := h.queries.CreatePreference(r.Context(), db.CreatePreferenceParams{
		ID:             uuid.New(),
		UserID:         userID,
		PreferenceType: req.PreferenceType,
		Label:          strings.TrimSpace(req.Label),
		Sentiment:      req.Sentiment,
		Weight:         req.Weight,
		IsHardGate:     req.IsHardGate,
		ContextType:    req.ContextType,
		Notes:          req.Notes,
		Aliases:        cleanAliases(req.Aliases),
	})
	if err != nil {
		if isUniqueViolation(err) {
			httputil.WriteError(w, http.StatusConflict, "already_exists",
				"a preference of this type already carries that label")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create preference")
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, pref)
}

func (h *PreferenceHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "preference id must be a valid UUID")
		return
	}

	var req preferenceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", msg)
		return
	}

	pref, err := h.queries.UpdatePreference(r.Context(), db.UpdatePreferenceParams{
		ID:             id,
		UserID:         userID,
		PreferenceType: req.PreferenceType,
		Label:          strings.TrimSpace(req.Label),
		Sentiment:      req.Sentiment,
		Weight:         req.Weight,
		IsHardGate:     req.IsHardGate,
		ContextType:    req.ContextType,
		Notes:          req.Notes,
		Aliases:        cleanAliases(req.Aliases),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "preference not found")
			return
		}
		if isUniqueViolation(err) {
			httputil.WriteError(w, http.StatusConflict, "already_exists",
				"a preference of this type already carries that label")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update preference")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, pref)
}

func (h *PreferenceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "preference id must be a valid UUID")
		return
	}

	rows, err := h.queries.DeletePreference(r.Context(), db.DeletePreferenceParams{ID: id, UserID: userID})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete preference")
		return
	}
	if rows == 0 {
		httputil.WriteError(w, http.StatusNotFound, "not_found", "preference not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
