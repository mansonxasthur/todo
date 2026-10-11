package todo

import (
	"context"
	"errors"
	"time"
)

type Todo struct {
	ID        int64      `json:"id"`
	Title     string     `json:"title"`
	Done      bool       `json:"done"`
	CreatedAt time.Time  `json:"created_at"`
	DoneAt    *time.Time `json:"done_at"`
}

const (
	MinTitleLen = 3
	MaxTitleLen = 255
)

var ErrNotFound = errors.New("todo not found")

// Repo is the persistence boundary for todos. Callers depend on this
// interface, not on pgx, so the implementation can be swapped or faked.
type Repo interface {
	Create(ctx context.Context, title string) (Todo, error)
	Get(ctx context.Context, id int64) (Todo, error)
	List(ctx context.Context) ([]Todo, error)
	// Update changes only the fields that are non-nil.
	Update(ctx context.Context, id int64, title *string, done *bool) (Todo, error)
	Delete(ctx context.Context, id int64) error
}
