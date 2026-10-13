// Package health is for checking service health and readiness
package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"example.com/todo/internal/server"
)

type Pinger interface {
	Ping(context.Context) error
}

type Handler struct {
	p   Pinger
	log *slog.Logger
}

type healthCheck struct {
	Status string `json:"status"`
}

func NewHandler(p Pinger, log *slog.Logger) *Handler {
	return &Handler{p: p, log: log}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.live)
	mux.HandleFunc("GET /readyz", h.ready)
}

func (h *Handler) live(w http.ResponseWriter, r *http.Request) {
	server.Success(w, healthCheck{Status: "OK"}, http.StatusOK)
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	pingContext, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.p.Ping(pingContext); err != nil {
		h.log.ErrorContext(r.Context(), "readiness check failed", "error", err)
		server.Failure(w, server.ErrUnavailable)
		return
	}

	server.Success(w, healthCheck{Status: "OK"}, http.StatusOK)
}
