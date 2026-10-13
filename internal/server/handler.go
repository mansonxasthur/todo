package server

import (
	"log/slog"
	"net/http"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux)
}

func CloseRequestBody(log *slog.Logger, r *http.Request) {
	if err := r.Body.Close(); err != nil {
		log.ErrorContext(r.Context(), "closing request body", "error", err)
	}
}
