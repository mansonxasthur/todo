package todo

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"example.com/todo/internal/server"
	"example.com/todo/internal/validation"
)

type Handler struct {
	repo Repo
	log  *slog.Logger
}

func NewHandler(repo Repo, log *slog.Logger) *Handler {
	return &Handler{repo: repo, log: log}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /todos", h.createTodo)
	mux.HandleFunc("GET /todos", h.listTodos)
	mux.HandleFunc("GET /todos/{id}", h.getTodo)
	mux.HandleFunc("PATCH /todos/{id}", h.updateTodo)
	mux.HandleFunc("DELETE /todos/{id}", h.deleteTodo)
}

func (h *Handler) createTodo(w http.ResponseWriter, r *http.Request) {
	var todo Todo

	body := r.Body

	defer r.Body.Close()

	err := json.NewDecoder(body).Decode(&todo)

	if err != nil {
		server.Failure(w, server.ErrInvalidBody)
		return
	}

	title := strings.TrimSpace(todo.Title)

	var errs validation.Errors
	errs.Add("title", validation.String(title, MinTitleLen, MaxTitleLen))

	if len(errs) > 0 {
		server.Failure(w, server.NewValidationError(errs))
		return
	}

	todo, err = h.repo.Create(r.Context(), title)

	if err != nil {
		h.log.Error("creating todo", "error", err)
		server.Failure(w, server.ErrInternal)
		return
	}

	server.Success(w, todo, http.StatusCreated)
}

func (h *Handler) listTodos(w http.ResponseWriter, r *http.Request) {
	todos, err := h.repo.List(r.Context())

	if err != nil {
		h.log.Error("listing todos", "error", err)
		server.Failure(w, server.ErrInternal)
		return
	}

	// A nil slice would encode as null; clients expect an array.
	if todos == nil {
		todos = []Todo{}
	}

	server.Success(w, todos, http.StatusOK)
}

func (h *Handler) getTodo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil {
		h.log.Error("converting resource id", "error", err)
		server.Failure(w, server.ErrResourceID)
		return
	}

	t, err := h.repo.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			server.Failure(w, server.ErrNotFound)
			return
		}

		h.log.Error("getting todo by id", "error", err)
		server.Failure(w, server.ErrInternal)
		return
	}

	server.Success(w, t, http.StatusOK)
}

// updateRequest uses pointers so a field that was not sent (nil) can be told
// apart from one sent as its zero value (false, "").
type updateRequest struct {
	Title *string `json:"title"`
	Done  *bool   `json:"done"`
}

func (h *Handler) updateTodo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil {
		server.Failure(w, server.ErrResourceID)
		return
	}

	defer r.Body.Close()

	var req updateRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.Failure(w, server.ErrInvalidBody)
		return
	}

	if req.Title == nil && req.Done == nil {
		server.Failure(w, server.ErrInvalidBody)
		return
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		req.Title = &title

		var errs validation.Errors
		errs.Add("title", validation.String(title, MinTitleLen, MaxTitleLen))

		if len(errs) > 0 {
			server.Failure(w, server.NewValidationError(errs))
			return
		}
	}

	t, err := h.repo.Update(r.Context(), id, req.Title, req.Done)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			server.Failure(w, server.ErrNotFound)
			return
		}

		h.log.Error("updating todo", "error", err)
		server.Failure(w, server.ErrInternal)
		return
	}

	server.Success(w, t, http.StatusOK)
}

func (h *Handler) deleteTodo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil {
		server.Failure(w, server.ErrResourceID)
		return
	}

	err = h.repo.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			server.Failure(w, server.ErrNotFound)
			return
		}
		server.Failure(w, server.ErrInternal)
		return
	}

	server.Success(w, nil, http.StatusNoContent)
}
