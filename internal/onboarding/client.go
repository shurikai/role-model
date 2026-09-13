// Package onboarding is the HTTP client for the onboarding-agent service
// (#117): a LangGraph interview that writes career data through this API's
// own REST endpoints, on the caller's behalf.
//
// Shaped like internal/renderer.Client on purpose -- a bounded-timeout HTTP
// client wrapping one endpoint on an internal Python service, with a typed
// error on non-2xx. The one difference from the renderer is auth: the
// renderer is stateless and unauthenticated, while a turn here needs to act
// as a specific user, so the caller's own bearer token is relayed in the
// request body on every call. This client never verifies or stores it --
// an invalid or expired token simply fails wherever the onboarding service
// tries to use it against this same API, the same 401 any other client
// would get.
package onboarding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RequestTimeout bounds a single /turns call. Generous relative to the
// renderer's: a turn can involve more than one LLM call (question phrasing
// plus tag extraction) before it pauses on the next question.
const RequestTimeout = 60 * time.Second

// Client calls the onboarding-agent service's /turns endpoint.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: RequestTimeout},
	}
}

// TurnError wraps a non-2xx response from the onboarding service, so callers
// can distinguish "service reachable but rejected the input" from network/
// connection failures.
type TurnError struct {
	StatusCode int
	Body       string
}

func (e *TurnError) Error() string {
	return fmt.Sprintf("onboarding agent returned status %d: %s", e.StatusCode, e.Body)
}

// TurnResponse is what the onboarding service returns for one turn.
type TurnResponse struct {
	SessionID string `json:"session_id"`
	Reply     string `json:"reply"`
	Done      bool   `json:"done"`
}

type turnRequest struct {
	Token     string  `json:"token"`
	SessionID *string `json:"session_id,omitempty"`
	Message   *string `json:"message,omitempty"`
}

// SendTurn sends one message in an interview and returns the agent's reply.
// sessionID and message are both nil to start a new interview; both are set
// on every turn after the first, echoing back the session_id the previous
// call returned.
func (c *Client) SendTurn(ctx context.Context, token string, sessionID, message *string) (TurnResponse, error) {
	body, err := json.Marshal(turnRequest{Token: token, SessionID: sessionID, Message: message})
	if err != nil {
		return TurnResponse{}, fmt.Errorf("marshal turn request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/turns", bytes.NewReader(body))
	if err != nil {
		return TurnResponse{}, fmt.Errorf("build turn request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return TurnResponse{}, fmt.Errorf("call onboarding agent: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return TurnResponse{}, fmt.Errorf("read onboarding agent response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TurnResponse{}, &TurnError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var turn TurnResponse
	if err := json.Unmarshal(respBody, &turn); err != nil {
		return TurnResponse{}, fmt.Errorf("parse onboarding agent response: %w", err)
	}
	return turn, nil
}
