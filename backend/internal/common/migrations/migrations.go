package migrations

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func Run(database *sql.DB, migrationsDir string) error {
	driver, err := postgres.WithInstance(database, &postgres.Config{})
	if err != nil {
		return err
	}

	migrator, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsDir),
		"postgres", driver)
	if err != nil {
		return err
	}

	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}
