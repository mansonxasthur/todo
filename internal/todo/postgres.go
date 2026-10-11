package todo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Scan fills destinations by position, so the order of these columns must
// match the order of the pointers in every Scan call below.
const columns = "id, title, done, created_at, done_at"

type PostgresRepo struct {
	db *pgxpool.Pool
}

func NewPostgresRepo(db *pgxpool.Pool) *PostgresRepo {
	return &PostgresRepo{db: db}
}

func (r *PostgresRepo) Create(ctx context.Context, title string) (Todo, error) {
	var t Todo
	err := r.db.QueryRow(ctx,
		`INSERT INTO todos (title) VALUES ($1) RETURNING `+columns, title,
	).Scan(&t.ID, &t.Title, &t.Done, &t.CreatedAt, &t.DoneAt)
	return t, err
}

func (r *PostgresRepo) Get(ctx context.Context, id int64) (Todo, error) {
	var t Todo
	err := r.db.QueryRow(ctx,
		`SELECT `+columns+` FROM todos WHERE id = $1`, id,
	).Scan(&t.ID, &t.Title, &t.Done, &t.CreatedAt, &t.DoneAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Todo{}, ErrNotFound
	}
	return t, err
}

func (r *PostgresRepo) List(ctx context.Context) ([]Todo, error) {
	rows, err := r.db.Query(ctx, `SELECT `+columns+` FROM todos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	// Always close rows, otherwise the connection is not returned to the pool.
	defer rows.Close()

	// Start with an empty slice so an empty table encodes as [] and not null.
	todos := []Todo{}
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Done, &t.CreatedAt, &t.DoneAt); err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}

	// Next returns false on error too, so check for one after the loop.
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return todos, nil
}

// Update changes only the non-nil arguments. A nil pointer is sent as NULL,
// and COALESCE keeps the current column value in that case.
func (r *PostgresRepo) Update(ctx context.Context, id int64, title *string, done *bool) (Todo, error) {
	var t Todo
	err := r.db.QueryRow(ctx,
		`UPDATE todos SET
		   title   = COALESCE($2, title),
		   done    = COALESCE($3, done),
		   done_at = CASE
		     WHEN $3::boolean IS NULL THEN done_at
		     WHEN $3 THEN COALESCE(done_at, now())
		     ELSE NULL
		   END
		 WHERE id = $1 RETURNING `+columns, id, title, done,
	).Scan(&t.ID, &t.Title, &t.Done, &t.CreatedAt, &t.DoneAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Todo{}, ErrNotFound
	}
	return t, err
}

func (r *PostgresRepo) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM todos WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
