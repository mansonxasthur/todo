package server

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	_, err := w.Write(body)
	if err != nil {
		fmt.Printf("error writing to body: %v\n", err)
	}
}
