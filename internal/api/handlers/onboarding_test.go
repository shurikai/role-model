package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/shurikai/role-model/internal/httputil"
	"github.com/shurikai/role-model/internal/onboarding"
)

// No DB dependency here — unlike every other handler test in this package,
// which needs a live database and is gated behind the integration build tag.

func TestOnboardingHandlerTurnRelaysTheBearerTokenWithoutTheBearerPrefix(t *testing.T) {
	var gotAuthHeader string
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotAuthHeader = body.Token
		_ = json.NewEncoder(w).Encode(onboarding.TurnResponse{
			SessionID: "s1", Reply: "What company did you work at?", Done: false,
		})
	}))
	defer agent.Close()

	h := NewOnboardingHandler(onboarding.NewClient(agent.URL))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/turns", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer the-users-jwt")
	req = req.WithContext(httputil.WithUserID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()

	h.Turn(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if gotAuthHeader != "the-users-jwt" {
		t.Errorf("relayed token = %q, want %q (Bearer prefix should be stripped)", gotAuthHeader, "the-users-jwt")
	}

	var resp onboarding.TurnResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.SessionID != "s1" || resp.Done {
		t.Errorf("response = %+v, want session s1, not done", resp)
	}
}

func TestOnboardingHandlerTurnPassesThroughSessionIDAndMessage(t *testing.T) {
	var gotBody struct {
		SessionID *string `json:"session_id"`
		Message   *string `json:"message"`
	}
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(onboarding.TurnResponse{SessionID: "s1", Reply: "ok", Done: false})
	}))
	defer agent.Close()

	h := NewOnboardingHandler(onboarding.NewClient(agent.URL))

	body := []byte(`{"session_id":"s1","message":"Acme Corp"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/turns", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer the-users-jwt")
	req = req.WithContext(httputil.WithUserID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()

	h.Turn(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if gotBody.SessionID == nil || *gotBody.SessionID != "s1" {
		t.Errorf("session_id = %v, want s1", gotBody.SessionID)
	}
	if gotBody.Message == nil || *gotBody.Message != "Acme Corp" {
		t.Errorf("message = %v, want Acme Corp", gotBody.Message)
	}
}

func TestOnboardingHandlerTurnReturns502WhenTheAgentIsUnreachable(t *testing.T) {
	// A client pointed at a URL nothing is listening on, rather than a
	// started-then-closed server, so the connection fails immediately
	// instead of racing a slow OS-level timeout.
	h := NewOnboardingHandler(onboarding.NewClient("http://127.0.0.1:1"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/turns", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer the-users-jwt")
	req = req.WithContext(httputil.WithUserID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()

	h.Turn(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Code != "onboarding_unreachable" {
		t.Errorf("code = %q, want onboarding_unreachable (connection never completed)", body.Code)
	}
}

// The distinction TestOnboardingHandlerTurnReturns502WhenTheAgentIsUnreachable
// checks the other side of: the agent was reached and answered with its own
// error (e.g. a resumed session_id predating a checkpoint reset), which used
// to collapse into the identical "failed to reach the onboarding agent"
// message — actively misleading, since the agent was never unreachable.
func TestOnboardingHandlerTurnDistinguishesAnAgentErrorFromUnreachable(t *testing.T) {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"detail":"the interview could not continue"}`))
	}))
	defer agent.Close()

	h := NewOnboardingHandler(onboarding.NewClient(agent.URL))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/turns", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer the-users-jwt")
	req = req.WithContext(httputil.WithUserID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()

	h.Turn(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "onboarding_failed" {
		t.Errorf("code = %q, want onboarding_failed (agent was reached)", body.Code)
	}
	// The agent's own raw response body must not leak through verbatim.
	if strings.Contains(body.Error, "the interview could not continue") {
		t.Errorf("response echoed the agent's raw body: %q", body.Error)
	}
}
