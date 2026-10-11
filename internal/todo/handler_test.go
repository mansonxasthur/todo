package todo_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/todo/internal/server"
	"example.com/todo/internal/todo"
)

// fakeRepo embeds the interface so only the methods a test needs are
// implemented. Calling any other method panics on the nil embedded value.
type fakeRepo struct {
	todo.Repo
	create func(ctx context.Context, title string) (todo.Todo, error)
	list   func(ctx context.Context) ([]todo.Todo, error)
	get    func(ctx context.Context, id int64) (todo.Todo, error)
	update func(ctx context.Context, id int64, title *string, done *bool) (todo.Todo, error)
}

func (f fakeRepo) Update(ctx context.Context, id int64, title *string, done *bool) (todo.Todo, error) {
	return f.update(ctx, id, title, done)
}

func (f fakeRepo) Get(ctx context.Context, id int64) (todo.Todo, error) {
	return f.get(ctx, id)
}

func (f fakeRepo) Create(ctx context.Context, title string) (todo.Todo, error) {
	return f.create(ctx, title)
}

func (f fakeRepo) List(ctx context.Context) ([]todo.Todo, error) {
	return f.list(ctx)
}

type envelope struct {
	Data  any `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details []struct {
			Field   string `json:"field"`
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"details"`
	} `json:"error"`
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) envelope {
	t.Helper()

	var resp envelope
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	return resp
}

// serve routes a request through the handler's real mux registration.
func serve(t *testing.T, repo todo.Repo, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	mux := http.NewServeMux()
	todo.NewHandler(repo, slog.New(slog.NewTextHandler(io.Discard, nil))).RegisterRoutes(mux)

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestCreateTodo(t *testing.T) {
	var gotTitle string
	repo := fakeRepo{create: func(_ context.Context, title string) (todo.Todo, error) {
		gotTitle = title
		return todo.Todo{ID: 1, Title: title}, nil
	}}

	rec := serve(t, repo, http.MethodPost, "/todos", `{"title":"buy milk"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if gotTitle != "buy milk" {
		t.Fatalf("repo got title %q, want %q", gotTitle, "buy milk")
	}

	resp := decode(t, rec)

	data := resp.Data.(map[string]any)

	idF, ok := data["id"].(float64)

	if !ok {
		t.Fatal("id is not float64")
	}

	id := int(idF)

	if id != 1 || data["title"] != "buy milk" || resp.Error != nil {
		t.Fatalf("unexpected response: err(%+v), data(%v)", resp.Error, data)
	}
}

func TestCreateTodoMalformedJSON(t *testing.T) {
	called := false
	repo := fakeRepo{create: func(context.Context, string) (todo.Todo, error) {
		called = true
		return todo.Todo{}, nil
	}}

	rec := serve(t, repo, http.MethodPost, "/todos", `{not json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if called {
		t.Fatal("repo was called for an invalid body")
	}

	resp := decode(t, rec)
	if resp.Error == nil || resp.Error.Code != server.ErrInvalidBody.Code || resp.Error.Message != server.ErrInvalidBody.Message {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestCreateTodoRepoError(t *testing.T) {
	repo := fakeRepo{create: func(context.Context, string) (todo.Todo, error) {
		return todo.Todo{}, errors.New("db down")
	}}

	rec := serve(t, repo, http.MethodPost, "/todos", `{"title":"buy milk"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	resp := decode(t, rec)
	if resp.Error == nil || resp.Error.Code != server.ErrInternal.Code || resp.Error.Message != server.ErrInternal.Message {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if strings.Contains(rec.Body.String(), "db down") {
		t.Fatalf("response leaks the internal error: %s", rec.Body.String())
	}
}

func TestCreateTodoValidation(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode string
		wantMsg  string
	}{
		{"missing title", `{}`, "REQUIRED", "required"},
		{"blank title", `{"title":"   "}`, "REQUIRED", "required"},
		{"too short", `{"title":"ab"}`, "MIN_LENGTH", "min length"},
		{"too short after trim", `{"title":"  ab  "}`, "MIN_LENGTH", "min length"},
		{"too long", `{"title":"` + strings.Repeat("a", 256) + `"}`, "MAX_LENGTH", "max length"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			repo := fakeRepo{create: func(context.Context, string) (todo.Todo, error) {
				called = true
				return todo.Todo{}, nil
			}}

			rec := serve(t, repo, http.MethodPost, "/todos", tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if called {
				t.Fatal("repo was called for an invalid title")
			}

			resp := decode(t, rec)
			if resp.Error == nil ||
				resp.Error.Code != server.ErrValidation.Code ||
				resp.Error.Message != server.ErrValidation.Message {
				t.Fatalf("error = %+v, want %q", resp.Error, server.ErrValidation.Code)
			}
			if len(resp.Error.Details) != 1 {
				t.Fatalf("details = %+v, want exactly one", resp.Error.Details)
			}
			d := resp.Error.Details[0]
			if d.Field != "title" || d.Code != tt.wantCode || d.Message != tt.wantMsg {
				t.Fatalf("detail = %+v, want title/%s/%s", d, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

func TestCreateTodoTrimsTitle(t *testing.T) {
	var gotTitle string
	repo := fakeRepo{create: func(_ context.Context, title string) (todo.Todo, error) {
		gotTitle = title
		return todo.Todo{ID: 1, Title: title}, nil
	}}

	rec := serve(t, repo, http.MethodPost, "/todos", `{"title":"  buy milk  "}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if gotTitle != "buy milk" {
		t.Fatalf("repo got %q, want %q", gotTitle, "buy milk")
	}
}

func TestListTodosSuccessfully(t *testing.T) {
	repo := fakeRepo{list: func(context.Context) ([]todo.Todo, error) {
		return []todo.Todo{
			{ID: 1, Title: "something new"},
			{ID: 2, Title: "something old"},
		}, nil
	}}

	rec := serve(t, repo, http.MethodGet, "/todos", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	resp := decode(t, rec)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	data := resp.Data.([]any)
	if len(data) != 2 {
		t.Fatalf("want list of 2, got %d", len(data))
	}

	for i, want := range []struct {
		id    float64
		title string
	}{{1, "something new"}, {2, "something old"}} {
		item := data[i].(map[string]any)
		if item["id"] != want.id || item["title"] != want.title {
			t.Fatalf("item %d = %v, want id %v title %q", i, item, want.id, want.title)
		}
	}
}

func TestListTodosEmptyIsArray(t *testing.T) {
	// A nil slice would encode as null; the handler must send [].
	repo := fakeRepo{list: func(context.Context) ([]todo.Todo, error) {
		return nil, nil
	}}

	rec := serve(t, repo, http.MethodGet, "/todos", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	data, ok := decode(t, rec).Data.([]any)
	if !ok || len(data) != 0 {
		t.Fatalf("data = %#v, want empty array", data)
	}
}

func TestListTodosRepoError(t *testing.T) {
	repo := fakeRepo{list: func(context.Context) ([]todo.Todo, error) {
		return nil, errors.New("db down")
	}}

	rec := serve(t, repo, http.MethodGet, "/todos", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	resp := decode(t, rec)
	if resp.Error == nil || resp.Error.Code != server.ErrInternal.Code || resp.Error.Message != server.ErrInternal.Message {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if strings.Contains(rec.Body.String(), "db down") {
		t.Fatalf("response leaks the internal error: %s", rec.Body.String())
	}
}

func TestGetTodo(t *testing.T) {
	var gotID int64
	repo := fakeRepo{get: func(_ context.Context, id int64) (todo.Todo, error) {
		gotID = id
		return todo.Todo{ID: id, Title: "buy milk"}, nil
	}}

	rec := serve(t, repo, http.MethodGet, "/todos/42", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotID != 42 {
		t.Fatalf("repo got id %d, want 42", gotID)
	}

	resp := decode(t, rec)
	data, ok := resp.Data.(map[string]any)
	if !ok || resp.Error != nil {
		t.Fatalf("unexpected response: err(%+v), data(%v)", resp.Error, resp.Data)
	}
	if data["id"] != float64(42) || data["title"] != "buy milk" {
		t.Fatalf("unexpected data: %v", data)
	}
}

func TestGetTodoInvalidID(t *testing.T) {
	for _, id := range []string{"abc", "1.5", "-", "99999999999999999999"} {
		t.Run(id, func(t *testing.T) {
			called := false
			repo := fakeRepo{get: func(context.Context, int64) (todo.Todo, error) {
				called = true
				return todo.Todo{}, nil
			}}

			rec := serve(t, repo, http.MethodGet, "/todos/"+id, "")

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if called {
				t.Fatal("repo was called for an invalid id")
			}

			resp := decode(t, rec)
			if resp.Error == nil || resp.Error.Code != server.ErrResourceID.Code || resp.Error.Message != server.ErrResourceID.Message {
				t.Fatalf("unexpected error: %+v", resp.Error)
			}
		})
	}
}

func TestGetTodoNotFound(t *testing.T) {
	repo := fakeRepo{get: func(context.Context, int64) (todo.Todo, error) {
		return todo.Todo{}, todo.ErrNotFound
	}}

	rec := serve(t, repo, http.MethodGet, "/todos/1", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	resp := decode(t, rec)
	if resp.Error == nil || resp.Error.Code != server.ErrNotFound.Code || resp.Error.Message != server.ErrNotFound.Message {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestGetTodoRepoError(t *testing.T) {
	repo := fakeRepo{get: func(context.Context, int64) (todo.Todo, error) {
		return todo.Todo{}, errors.New("db down")
	}}

	rec := serve(t, repo, http.MethodGet, "/todos/1", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	resp := decode(t, rec)
	if resp.Error == nil || resp.Error.Code != server.ErrInternal.Code || resp.Error.Message != server.ErrInternal.Message {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if strings.Contains(rec.Body.String(), "db down") {
		t.Fatalf("response leaks the internal error: %s", rec.Body.String())
	}
}

func TestUpdateTodo(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantTitle *string
		wantDone  *bool
	}{
		{"done only", `{"done":true}`, nil, ptr(true)},
		{"undone", `{"done":false}`, nil, ptr(false)},
		{"title only", `{"title":"buy milk"}`, ptr("buy milk"), nil},
		{"title is trimmed", `{"title":"  buy milk  "}`, ptr("buy milk"), nil},
		{"both", `{"title":"buy milk","done":true}`, ptr("buy milk"), ptr(true)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotID int64
			var gotTitle *string
			var gotDone *bool
			repo := fakeRepo{update: func(_ context.Context, id int64, title *string, done *bool) (todo.Todo, error) {
				gotID, gotTitle, gotDone = id, title, done
				return todo.Todo{ID: id, Title: "buy milk"}, nil
			}}

			rec := serve(t, repo, http.MethodPatch, "/todos/7", tt.body)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if gotID != 7 {
				t.Fatalf("repo got id %d, want 7", gotID)
			}
			if !equalPtr(gotTitle, tt.wantTitle) || !equalPtr(gotDone, tt.wantDone) {
				t.Fatalf("repo got title=%v done=%v, want title=%v done=%v",
					deref(gotTitle), deref(gotDone), deref(tt.wantTitle), deref(tt.wantDone))
			}
			if resp := decode(t, rec); resp.Error != nil {
				t.Fatalf("unexpected error: %+v", resp.Error)
			}
		})
	}
}

func TestUpdateTodoBadRequests(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		body     string
		wantCode string
	}{
		{"invalid id", "/todos/abc", `{"done":true}`, server.ErrResourceID.Code},
		{"malformed json", "/todos/1", `{not json`, server.ErrInvalidBody.Code},
		{"nothing to update", "/todos/1", `{}`, server.ErrInvalidBody.Code},
		{"wrong type", "/todos/1", `{"done":"yes"}`, server.ErrInvalidBody.Code},
		{"title too short", "/todos/1", `{"title":"ab"}`, server.ErrValidation.Code},
		{"blank title", "/todos/1", `{"title":"   "}`, server.ErrValidation.Code},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			repo := fakeRepo{update: func(context.Context, int64, *string, *bool) (todo.Todo, error) {
				called = true
				return todo.Todo{}, nil
			}}

			rec := serve(t, repo, http.MethodPatch, tt.path, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if called {
				t.Fatal("repo was called for an invalid request")
			}
			if resp := decode(t, rec); resp.Error == nil || resp.Error.Code != tt.wantCode {
				t.Fatalf("error = %+v, want code %q", resp.Error, tt.wantCode)
			}
		})
	}
}

func TestUpdateTodoNotFound(t *testing.T) {
	repo := fakeRepo{update: func(context.Context, int64, *string, *bool) (todo.Todo, error) {
		return todo.Todo{}, todo.ErrNotFound
	}}

	rec := serve(t, repo, http.MethodPatch, "/todos/1", `{"done":true}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if resp := decode(t, rec); resp.Error == nil || resp.Error.Code != server.ErrNotFound.Code {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestUpdateTodoRepoError(t *testing.T) {
	repo := fakeRepo{update: func(context.Context, int64, *string, *bool) (todo.Todo, error) {
		return todo.Todo{}, errors.New("db down")
	}}

	rec := serve(t, repo, http.MethodPatch, "/todos/1", `{"done":true}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if resp := decode(t, rec); resp.Error == nil || resp.Error.Code != server.ErrInternal.Code {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if strings.Contains(rec.Body.String(), "db down") {
		t.Fatalf("response leaks the internal error: %s", rec.Body.String())
	}
}

func ptr[T any](v T) *T { return &v }

func equalPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// deref makes pointers printable in failure messages.
func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
