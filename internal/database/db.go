package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

const migrationsDir = "db/migrations"
const migrationsDirEnv = "RCV_MIGRATIONS_DIR"

type migrationFile struct {
	version string
	name    string
	sql     string
}

// InitDB opens and verifies the database connection, returning an error so the caller can decide how to handle failures.
func InitDB(url string) (*sql.DB, error) {
	db, err := sql.Open("libsql", url)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	StartCleanupWorker(db)

	return db, nil
}

func StartCleanupWorker(db *sql.DB) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		if err := deleteExpiredPolls(db); err != nil {
			log.Printf("cleanup error: %v", err)
		}

		for range ticker.C {
			if err := deleteExpiredPolls(db); err != nil {
				log.Printf("cleanup error: %v", err)
			}
		}
	}()
}

func deleteExpiredPolls(db *sql.DB) error {
	res, err := db.Exec(`
		DELETE FROM polls
		WHERE created_at < datetime('now', '-14 days')
	`)
	if err != nil {
		return fmt.Errorf("delete expired polls: %w", err)
	}

	deleted, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("expired polls affected rows: %w", err)
	}

	if deleted > 0 {
		log.Printf("cleanup: deleted %d poll(s) older than 14 days", deleted)
	}

	return nil
}

// runMigrations applies SQL files in db/migrations once, in lexical order.
func runMigrations(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	dir, err := resolveMigrationsDir()
	if err != nil {
		return err
	}

	files, err := loadMigrationFiles(dir)
	if err != nil {
		return err
	}

	applied := map[string]struct{}{}
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("read applied migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate applied migrations: %w", err)
	}

	if len(files) > 0 && len(applied) == 0 {
		log.Printf("database: found %d migration file(s) in %q with no recorded applied versions; initial migration run may be starting", len(files), dir)
	}

	for _, m := range files {
		if _, alreadyApplied := applied[m.version]; alreadyApplied {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", m.name, err)
		}

		if _, err := tx.Exec(m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}

		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, name) VALUES (?, ?)`, m.version, m.name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m.name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.name, err)
		}

		log.Printf("database: applied migration %s (%s)", m.version, m.name)
	}

	return nil
}

func loadMigrationFiles(dir string) ([]migrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %q: %w", dir, err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".sql") {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	migrations := make([]migrationFile, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", path, err)
		}

		sqlText := strings.TrimSpace(string(b))
		if sqlText == "" {
			continue
		}

		migrations = append(migrations, migrationFile{
			version: strings.TrimSuffix(name, filepath.Ext(name)),
			name:    name,
			sql:     sqlText,
		})
	}

	return migrations, nil
}

func resolveMigrationsDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(migrationsDirEnv)); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
		return "", fmt.Errorf("%s points to a missing directory: %q", migrationsDirEnv, dir)
	}

	if info, err := os.Stat(migrationsDir); err == nil && info.IsDir() {
		return migrationsDir, nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path for migrations: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	candidate := filepath.Join(exeDir, migrationsDir)
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate, nil
	}

	return "", fmt.Errorf("could not find migrations directory; set %s or ensure %q exists", migrationsDirEnv, migrationsDir)
}
