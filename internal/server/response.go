package server

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type responseWrapper struct {
	Data  any       `json:"data"`
	Error *APIError `json:"error"`
}

func Success(w http.ResponseWriter, body any, status int) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(responseWrapper{Data: body}); err != nil {
		Failure(w, ErrInternal)
		return
	}
	writeJSON(w, buf.Bytes(), status)
}

// Failure writes e as the error response, using its status.
func Failure(w http.ResponseWriter, e APIError) {
	body, _ := json.Marshal(responseWrapper{Error: &e})
	writeJSON(w, body, e.Status)
}

func writeJSON(w http.ResponseWriter, body []byte, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Headers are already sent, and a failed write means the client went
	// away, so there is nothing to recover or worth logging.
	_, _ = w.Write(body)
}
