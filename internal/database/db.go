package database

import (
	"database/sql"
	"fmt"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

// InitDB opens and verifies the database connection, returning an error so the caller can decide how to handle failures.
func InitDB(url string) (*sql.DB, error) {
	db, err := sql.Open("libsql", url)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

// runMigrations applies the schema idempotently on every startup.
func runMigrations(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS polls (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		key         TEXT    UNIQUE NOT NULL,
		title       TEXT    NOT NULL,
		description TEXT    NOT NULL DEFAULT '',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS candidates (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		poll_id       INTEGER NOT NULL REFERENCES polls(id),
		name          TEXT    NOT NULL,
		display_order INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS ballots (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		poll_id    INTEGER NOT NULL REFERENCES polls(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ballot_rankings (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		ballot_id    INTEGER NOT NULL REFERENCES ballots(id),
		candidate_id INTEGER NOT NULL REFERENCES candidates(id),
		rank         INTEGER NOT NULL
	);
	`

	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// Additive migrations — ALTER TABLE ignores "duplicate column" errors so
	// this is safe to run on every startup against an existing database.
	additiveMigrations := []string{
		`ALTER TABLE polls ADD COLUMN closed_at DATETIME`,
	}
	for _, m := range additiveMigrations {
		// Ignore errors; the only expected failure is "duplicate column name".
		_, _ = db.Exec(m)
	}

	return nil
}
