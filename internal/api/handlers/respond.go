package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json response: %v", err)
	}
}

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: message, Code: code})
}
