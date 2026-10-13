package server

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.home)
	mux.HandleFunc("GET /cats", s.cats)

	for _, h := range s.handlers {
		h.RegisterRoutes(mux)
	}
	return otelhttp.NewHandler(
		s.logRequests(
			s.recoverPanic(mux),
		),
		"http.server",
	)
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello friend!"))
}

type cat struct {
	Breed string `json:"breed"`
	Name  string `json:"name"`
}

func (s *Server) cats(w http.ResponseWriter, r *http.Request) {
	cats := []cat{
		{Breed: "Siami", Name: "Lulu"},
		{Breed: "Shiraz", Name: "Yumi"},
	}
	Success(w, cats, http.StatusOK)
}
