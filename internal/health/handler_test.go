package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/todo/internal/health"
)

type fakePinger struct {
	err error
}

func (f fakePinger) Ping(context.Context) error {
	return f.err
}

type envelope struct {
	Data struct {
		Status string `json:"status"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// serve routes a request through the handler's real mux registration.
func serve(t *testing.T, p health.Pinger, method, path string) *httptest.ResponseRecorder {
	t.Helper()

	mux := http.NewServeMux()
	health.NewHandler(p, slog.New(slog.NewTextHandler(io.Discard, nil))).RegisterRoutes(mux)

	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestEndpoints(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		pingErr    error
		wantStatus int
		wantCode   string // expected error code, empty on success
	}{
		{name: "live ok", path: "/healthz", wantStatus: http.StatusOK},
		{name: "live ignores database failure", path: "/healthz", pingErr: errors.New("db down"), wantStatus: http.StatusOK},
		{name: "ready ok", path: "/readyz", wantStatus: http.StatusOK},
		{name: "ready database down", path: "/readyz", pingErr: errors.New("db down"), wantStatus: http.StatusServiceUnavailable, wantCode: "SERVICE_UNAVAILABLE"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(t, fakePinger{err: tc.pingErr}, http.MethodGet, tc.path)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			var resp envelope
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decoding response: %v", err)
			}

			if tc.wantCode == "" {
				if resp.Error != nil {
					t.Fatalf("unexpected error: %+v", resp.Error)
				}
				if resp.Data.Status != "OK" {
					t.Errorf("status field = %q, want OK", resp.Data.Status)
				}
				return
			}

			if resp.Error == nil {
				t.Fatal("expected error in response, got nil")
			}
			if resp.Error.Code != tc.wantCode {
				t.Errorf("error code = %q, want %q", resp.Error.Code, tc.wantCode)
			}
			// The dependency's raw error must never reach the client.
			if resp.Error.Message != "service unavailable" {
				t.Errorf("error message = %q, want generic message", resp.Error.Message)
			}
		})
	}
}

func TestMethodNotAllowed(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz"} {
		rec := serve(t, fakePinger{}, http.MethodPost, path)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s status = %d, want %d", path, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}
