package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"time"
)

const shutdownTimeout = 10 * time.Second

type Server struct {
	log        *slog.Logger
	http       *http.Server
	handlerMap map[string]struct{}
	handlers   []Handler
}

func New(addr string, log *slog.Logger, handlers []Handler) *Server {
	s := &Server{
		log:        log,
		handlerMap: make(map[string]struct{}),
	}

	for _, h := range handlers {
		s.AddHandler(h)
	}

	s.http = &http.Server{
		Addr:              addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return s
}

func (s *Server) AddHandler(handler Handler) {
	t := reflect.TypeOf(handler)

	if _, exists := s.handlerMap[t.String()]; exists {
		return
	}

	s.handlers = append(s.handlers, handler)
	s.handlerMap[t.String()] = struct{}{}
}

// Run serves until ctx is cancelled, then drains in-flight requests.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("server listening", "addr", s.http.Addr)
		if err := s.http.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	s.log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return <-errCh
}
