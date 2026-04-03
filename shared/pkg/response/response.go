package response

import (
	"encoding/json"
	"net/http"
)

// JSON writes v as a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Error writes a structured error response: {"error":"...","code":"..."}.
func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, errorBody{Error: message, Code: code})
}
