package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/httputil"
)

// FeedbackHandler takes product feedback from inside the app (#167): "this
// screen confused me", not feedback on a generated bullet, which is
// contribution_feedback's job.
type FeedbackHandler struct {
	queries *db.Queries
}

func NewFeedbackHandler(queries *db.Queries) *FeedbackHandler {
	return &FeedbackHandler{queries: queries}
}

// The limits mirror app_feedback's CHECKs (message) and keep the two
// client-supplied context fields to what a pathname and a build SHA can be.
// Lengths are counted in characters, as Postgres length() counts them.
const (
	maxFeedbackMessage    = 4000
	maxFeedbackRoute      = 500
	maxFeedbackAppVersion = 64
)

var feedbackCategories = map[string]bool{"bug": true, "confusing": true, "idea": true}

type createFeedbackRequest struct {
	Category   string  `json:"category"`
	Message    string  `json:"message"`
	Route      string  `json:"route"`
	AppVersion string  `json:"app_version"`
	Viewport   *string `json:"viewport"`
	// Accepted so a body carrying one is not refused by decodeJSON's
	// DisallowUnknownFields, and never read: the owner is the authenticated
	// user, whatever the body claims.
	UserID json.RawMessage `json:"user_id"`
}

type createFeedbackResponse struct {
	ID uuid.UUID `json:"id"`
}

// Create stores one report. The client sends where the tester was (route)
// and which build they were on (app_version) so they never have to type
// either; the user agent is read from the request header, not the body.
func (h *FeedbackHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	var req createFeedbackRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	message := strings.TrimSpace(req.Message)
	route := strings.TrimSpace(req.Route)
	appVersion := strings.TrimSpace(req.AppVersion)

	switch {
	case !feedbackCategories[req.Category]:
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "category must be one of bug, confusing, idea")
		return
	case message == "":
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "message is required")
		return
	case utf8.RuneCountInString(message) > maxFeedbackMessage:
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "message must be at most 4000 characters")
		return
	case route == "":
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "route is required")
		return
	case utf8.RuneCountInString(route) > maxFeedbackRoute:
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "route must be at most 500 characters")
		return
	case appVersion == "":
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "app_version is required")
		return
	case utf8.RuneCountInString(appVersion) > maxFeedbackAppVersion:
		httputil.WriteError(w, http.StatusBadRequest, "validation_error", "app_version must be at most 64 characters")
		return
	}

	feedback, err := h.queries.CreateAppFeedback(r.Context(), db.CreateAppFeedbackParams{
		ID:         uuid.New(),
		UserID:     userID,
		Category:   req.Category,
		Message:    message,
		Route:      route,
		AppVersion: appVersion,
		UserAgent:  optionalText(r.UserAgent()),
		Viewport:   trimmedOrNil(req.Viewport),
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to save feedback")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, createFeedbackResponse{ID: feedback.ID})
}

func optionalText(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func trimmedOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	return optionalText(*s)
}
