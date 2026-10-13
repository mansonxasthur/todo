package database

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"example.com/todo/migrations"
)

// Migrate applies all pending up-migrations. Already-applied versions are
// tracked by golang-migrate in the schema_migrations table.
func Migrate(url string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	// The pgx v5 driver registers under the "pgx5" scheme.
	dbURL := strings.Replace(url, "postgres://", "pgx5://", 1)

	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer func() {
		sourceErr, databaseErr := m.Close()

		if sourceErr != nil {
			fmt.Printf("source error: %v\n", sourceErr)
		}

		if databaseErr != nil {
			fmt.Printf("database error: %v\n", databaseErr)
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}
