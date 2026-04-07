package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDeletePollCascadesAssociatedRows(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	migrationsPath, err := testMigrationsPath()
	if err != nil {
		t.Fatalf("resolve migrations path: %v", err)
	}

	oldMigrationsDir := os.Getenv(migrationsDirEnv)
	if err := os.Setenv(migrationsDirEnv, migrationsPath); err != nil {
		t.Fatalf("set %s: %v", migrationsDirEnv, err)
	}
	defer func() {
		if oldMigrationsDir == "" {
			_ = os.Unsetenv(migrationsDirEnv)
			return
		}
		_ = os.Setenv(migrationsDirEnv, oldMigrationsDir)
	}()

	if err := runMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	res, err := db.Exec(`INSERT INTO polls (key, title, description) VALUES (?, ?, ?)`, "pollkey01", "Test Poll", "desc")
	if err != nil {
		t.Fatalf("insert poll: %v", err)
	}
	pollID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("poll id: %v", err)
	}

	res, err = db.Exec(`INSERT INTO candidates (poll_id, name, display_order) VALUES (?, ?, ?)`, pollID, "A", 1)
	if err != nil {
		t.Fatalf("insert candidate A: %v", err)
	}
	candA, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("candidate A id: %v", err)
	}

	res, err = db.Exec(`INSERT INTO candidates (poll_id, name, display_order) VALUES (?, ?, ?)`, pollID, "B", 2)
	if err != nil {
		t.Fatalf("insert candidate B: %v", err)
	}
	candB, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("candidate B id: %v", err)
	}

	res, err = db.Exec(`INSERT INTO ballots (poll_id) VALUES (?)`, pollID)
	if err != nil {
		t.Fatalf("insert ballot: %v", err)
	}
	ballotID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("ballot id: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO ballot_rankings (ballot_id, candidate_id, rank) VALUES (?, ?, ?)`, ballotID, candA, 1); err != nil {
		t.Fatalf("insert ranking A: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO ballot_rankings (ballot_id, candidate_id, rank) VALUES (?, ?, ?)`, ballotID, candB, 2); err != nil {
		t.Fatalf("insert ranking B: %v", err)
	}

	if _, err := db.Exec(`DELETE FROM polls WHERE id = ?`, pollID); err != nil {
		t.Fatalf("delete poll: %v", err)
	}

	assertTableRowCount(t, db, "polls", 0)
	assertTableRowCount(t, db, "candidates", 0)
	assertTableRowCount(t, db, "ballots", 0)
	assertTableRowCount(t, db, "ballot_rankings", 0)
}

func assertTableRowCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()

	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("table %s row count = %d, want %d", table, got, want)
	}
}

func testMigrationsPath() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	return filepath.Join(repoRoot, "db", "migrations"), nil
}
