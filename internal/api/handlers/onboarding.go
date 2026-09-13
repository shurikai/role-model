package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/shurikai/role-model/internal/httputil"
	"github.com/shurikai/role-model/internal/onboarding"
)

// OnboardingHandler proxies conversational career-data entry (#117) to the
// internal onboarding-agent service. This is the only place the raw bearer
// token is read back off the request — RequireAuth reads it too, but only to
// parse the subject into context; it never removes the header, so it is
// still here for this handler to relay.
type OnboardingHandler struct {
	client *onboarding.Client
}

func NewOnboardingHandler(client *onboarding.Client) *OnboardingHandler {
	return &OnboardingHandler{client: client}
}

type turnRequest struct {
	SessionID *string `json:"session_id"`
	Message   *string `json:"message"`
}

// Turn sends one message in an interview and returns the agent's reply.
//
// The onboarding service has no auth of its own: this call is the only place
// a request for it is authenticated at all, by virtue of sitting behind
// RequireAuth like every other route in this group. The token relayed here
// is what lets the agent write career data back through this same API on the
// caller's behalf — see internal/onboarding.Client and onboarding-agent/README.md.
func (h *OnboardingHandler) Turn(w http.ResponseWriter, r *http.Request) {
	if _, ok := httputil.UserIDFromContext(r.Context()); !ok {
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing user context")
		return
	}

	// RequireAuth already validated this header before this handler ever
	// runs; it reads headers, it doesn't consume them, so the raw bearer
	// value is still here to relay. Stripped of "Bearer " the same way
	// RequireAuth itself parses it, so the onboarding service gets the bare
	// token and can put its own "Bearer " prefix on when it calls back —
	// relaying the header verbatim would double it up.
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		// Defensive, not a path any real request reaches.
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "missing authorization header")
		return
	}

	var req turnRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	turn, err := h.client.SendTurn(r.Context(), token, req.SessionID, req.Message)
	if err != nil {
		log.Printf("onboarding turn: %v", err)
		// A TurnError means the agent was reached and answered — with a
		// problem of its own, not this API's — as distinct from a genuine
		// connection failure. Collapsing both into "failed to reach" was
		// actively misleading: the agent can be up and every request still
		// fail this way if, say, a resumed session_id predates a checkpoint
		// reset. Neither case echoes the agent's raw response body; only that
		// it was reached.
		var turnErr *onboarding.TurnError
		if errors.As(err, &turnErr) {
			httputil.WriteError(w, http.StatusBadGateway, "onboarding_failed",
				"the onboarding agent could not continue this interview")
			return
		}
		httputil.WriteError(w, http.StatusBadGateway, "onboarding_unreachable", "failed to reach the onboarding agent")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, turn)
}
