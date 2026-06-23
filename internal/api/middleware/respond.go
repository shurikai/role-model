package middleware

import (
	"encoding/json"
	"log"
	"net/http"
)

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}{Error: message, Code: code}); err != nil {
		log.Printf("write error response: %v", err)
	}
}
