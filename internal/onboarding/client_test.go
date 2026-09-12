package onboarding

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func ptr(s string) *string { return &s }

func TestSendTurnStartsANewInterviewWithNoSessionOrMessage(t *testing.T) {
	var gotBody turnRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/turns" {
			t.Errorf("path = %q, want /turns", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(TurnResponse{
			SessionID: "s1", Reply: "What company did you work at?", Done: false,
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	turn, err := client.SendTurn(context.Background(), "the-jwt", nil, nil)
	if err != nil {
		t.Fatalf("SendTurn: %v", err)
	}

	if gotBody.Token != "the-jwt" {
		t.Errorf("relayed token = %q, want %q", gotBody.Token, "the-jwt")
	}
	if gotBody.SessionID != nil || gotBody.Message != nil {
		t.Errorf("starting a new interview sent session_id=%v message=%v, want both nil",
			gotBody.SessionID, gotBody.Message)
	}
	if turn.SessionID != "s1" || turn.Done {
		t.Errorf("turn = %+v, want session s1, not done", turn)
	}
}

func TestSendTurnEchoesSessionIDAndMessageOnResume(t *testing.T) {
	var gotBody turnRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(TurnResponse{SessionID: "s1", Reply: "Thanks.", Done: true})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	turn, err := client.SendTurn(context.Background(), "the-jwt", ptr("s1"), ptr("Acme Corp"))
	if err != nil {
		t.Fatalf("SendTurn: %v", err)
	}

	if gotBody.SessionID == nil || *gotBody.SessionID != "s1" {
		t.Errorf("session_id = %v, want s1", gotBody.SessionID)
	}
	if gotBody.Message == nil || *gotBody.Message != "Acme Corp" {
		t.Errorf("message = %v, want Acme Corp", gotBody.Message)
	}
	if !turn.Done {
		t.Error("Done = false, want true")
	}
}

func TestSendTurnWrapsANonTwoxxResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":"invalid request"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.SendTurn(context.Background(), "the-jwt", nil, nil)
	if err == nil {
		t.Fatal("SendTurn returned no error for a 422 response")
	}
	var turnErr *TurnError
	if !errors.As(err, &turnErr) {
		t.Fatalf("error = %v, want a *TurnError", err)
	}
	if turnErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("StatusCode = %d, want 422", turnErr.StatusCode)
	}
}
