package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// recoverServer returns a Server whose log output is captured in buf. It is
// an internal test because recoverPanic is unexported.
func recoverServer() (*Server, *bytes.Buffer) {
	var buf bytes.Buffer
	return &Server{log: slog.New(slog.NewJSONHandler(&buf, nil))}, &buf
}

func TestRecoverPanicReturnsInternalError(t *testing.T) {
	s, _ := recoverServer()
	h := s.recoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var resp struct {
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("error = %+v, want code INTERNAL_ERROR", resp.Error)
	}
	// The panic value is an internal detail and must never reach the client.
	if strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("response leaks panic value: %s", rec.Body.String())
	}
}

func TestRecoverPanicLogsValueAndReadableStack(t *testing.T) {
	s, buf := recoverServer()
	h := s.recoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/todos", nil))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("decoding log entry %q: %v", buf.String(), err)
	}

	if entry["error"] != "boom" {
		t.Errorf("error = %v, want boom", entry["error"])
	}
	if entry["method"] != http.MethodGet || entry["path"] != "/todos" {
		t.Errorf("method/path = %v %v, want GET /todos", entry["method"], entry["path"])
	}
	// A []byte stack would be base64-encoded by slog and fail this check.
	stack, _ := entry["stack"].(string)
	if !strings.Contains(stack, "recoverPanic") {
		t.Errorf("stack is not a readable trace: %q", stack)
	}
}

func TestRecoverPanicPropagatesErrAbortHandler(t *testing.T) {
	s, buf := recoverServer()
	h := s.recoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	var got any
	func() {
		defer func() { got = recover() }()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()

	if err, ok := got.(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
		t.Errorf("recovered %v, want http.ErrAbortHandler to propagate", got)
	}
	if buf.Len() != 0 {
		t.Errorf("deliberate abort was logged: %s", buf.String())
	}
}

func TestRecoverPanicPassesThroughNormalResponses(t *testing.T) {
	s, buf := recoverServer()
	h := s.recoverPanic(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		//nolint discard for testing
		w.Write([]byte("short and stout"))
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
	if rec.Body.String() != "short and stout" {
		t.Errorf("body = %q, want handler's body untouched", rec.Body.String())
	}
	if buf.Len() != 0 {
		t.Errorf("unexpected log output: %s", buf.String())
	}
}
