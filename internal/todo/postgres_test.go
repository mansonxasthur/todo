package todo_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"example.com/todo/internal/database"
	"example.com/todo/internal/todo"
)

var (
	poolOnce sync.Once
	pool     *pgxpool.Pool
	poolErr  error
	ctr      *postgres.PostgresContainer
)

// TestMain only tears down: the container starts lazily on first use (see
// testPool), so tests that don't touch the database, like the handler tests,
// stay fast and don't need Docker.
func TestMain(m *testing.M) {
	code := m.Run()

	if pool != nil {
		pool.Close()
	}
	if ctr != nil {
		if err := ctr.Terminate(context.Background()); err != nil {
			fmt.Printf("Error terminating postgres container: %v\n", err)
		}
	}
	os.Exit(code)
}

// testPool starts a throwaway Postgres container, applies the migrations and
// returns a pool shared by all repo tests in this package.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	poolOnce.Do(func() {
		ctx := context.Background()

		ctr, poolErr = postgres.Run(ctx, "postgres:17-alpine",
			postgres.WithDatabase("todo_test"),
			postgres.WithUsername("todo"),
			postgres.WithPassword("todo"),
			postgres.BasicWaitStrategies(),
		)
		if poolErr != nil {
			poolErr = fmt.Errorf("starting postgres container: %w", poolErr)
			return
		}

		var url string
		if url, poolErr = ctr.ConnectionString(ctx, "sslmode=disable"); poolErr != nil {
			return
		}
		if poolErr = database.Migrate(url); poolErr != nil {
			return
		}
		pool, poolErr = pgxpool.New(ctx, url)
	})

	if poolErr != nil {
		t.Fatalf("test database: %v", poolErr)
	}
	return pool
}

// newRepo returns a repo over an empty table with ids reset to 1.
// Tests share one database, so they must not call t.Parallel().
func newRepo(t *testing.T) *todo.PostgresRepo {
	t.Helper()

	db := testPool(t)
	if _, err := db.Exec(context.Background(), "TRUNCATE todos RESTART IDENTITY"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return todo.NewPostgresRepo(db)
}

func TestCreateAndGet(t *testing.T) {
	repo, ctx := newRepo(t), context.Background()

	created, err := repo.Create(ctx, "buy milk")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Title != "buy milk" || created.Done || created.DoneAt != nil {
		t.Fatalf("unexpected todo: %+v", created)
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID || got.Title != created.Title {
		t.Fatalf("got %+v, want %+v", got, created)
	}
}

func TestGetNotFound(t *testing.T) {
	repo := newRepo(t)

	_, err := repo.Get(context.Background(), 9999)
	if !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestListEmptyIsNotNil(t *testing.T) {
	repo := newRepo(t)

	todos, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if todos == nil || len(todos) != 0 {
		t.Fatalf("got %#v, want empty non-nil slice", todos)
	}
}

func TestListOrderedByID(t *testing.T) {
	repo, ctx := newRepo(t), context.Background()

	for _, title := range []string{"a", "b", "c"} {
		if _, err := repo.Create(ctx, title); err != nil {
			t.Fatalf("create %q: %v", title, err)
		}
	}

	todos, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(todos) != 3 {
		t.Fatalf("got %d todos, want 3", len(todos))
	}
	for i, want := range []string{"a", "b", "c"} {
		if todos[i].Title != want {
			t.Fatalf("todos[%d].Title = %q, want %q", i, todos[i].Title, want)
		}
	}
}

func TestListIncludesDoneState(t *testing.T) {
	repo, ctx := newRepo(t), context.Background()

	yes := true
	open, _ := repo.Create(ctx, "still open")
	finished, _ := repo.Create(ctx, "finished")
	if _, err := repo.Update(ctx, finished.ID, nil, &yes); err != nil {
		t.Fatalf("set done: %v", err)
	}

	todos, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("got %d todos, want 2", len(todos))
	}

	if got := todos[0]; got.ID != open.ID || got.Done || got.DoneAt != nil {
		t.Fatalf("open todo = %+v, want Done=false and DoneAt nil", got)
	}
	if got := todos[1]; got.ID != finished.ID || !got.Done || got.DoneAt == nil {
		t.Fatalf("finished todo = %+v, want Done=true and DoneAt set", got)
	}
}

func TestUpdate(t *testing.T) {
	repo, ctx := newRepo(t), context.Background()

	created, _ := repo.Create(ctx, "write tests")
	yes, no, title := true, false, "write more tests"

	updated, err := repo.Update(ctx, created.ID, nil, &yes)
	if err != nil {
		t.Fatalf("mark done: %v", err)
	}
	if !updated.Done || updated.DoneAt == nil || updated.Title != "write tests" {
		t.Fatalf("got %+v, want Done=true, DoneAt set, title unchanged", updated)
	}

	// Marking done again must not move the completion time.
	again, _ := repo.Update(ctx, created.ID, nil, &yes)
	if !again.DoneAt.Equal(*updated.DoneAt) {
		t.Fatalf("DoneAt changed from %v to %v", updated.DoneAt, again.DoneAt)
	}

	renamed, err := repo.Update(ctx, created.ID, &title, nil)
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Title != title || !renamed.Done || renamed.DoneAt == nil {
		t.Fatalf("got %+v, want new title and done state untouched", renamed)
	}

	reopened, err := repo.Update(ctx, created.ID, nil, &no)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.Done || reopened.DoneAt != nil || reopened.Title != title {
		t.Fatalf("got %+v, want Done=false, DoneAt nil, title unchanged", reopened)
	}

	if _, err := repo.Update(ctx, 9999, &title, &yes); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("missing id: got %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	repo, ctx := newRepo(t), context.Background()

	created, _ := repo.Create(ctx, "temp")

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.Delete(ctx, created.ID); !errors.Is(err, todo.ErrNotFound) {
		t.Fatalf("second delete: got %v, want ErrNotFound", err)
	}
}
