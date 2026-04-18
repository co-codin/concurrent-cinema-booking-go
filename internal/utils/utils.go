package utils

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSON encodes v as JSON with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

// WriteError writes an {"error": "..."} JSON payload.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

// DecodeJSON decodes a JSON body into v, returning false and writing 400 on failure.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Body == nil {
		WriteError(w, http.StatusBadRequest, "missing request body")
		return false
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return false
	}
	return true
}
