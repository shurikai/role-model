package handlers

import (
	"encoding/json"
	"net/http"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON: "+err.Error())
		return false
	}
	return true
}
