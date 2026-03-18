package database

import (
	"database/sql"
	"log"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

func InitDB(url string) *sql.DB {
	db, err := sql.Open("libsql", url)
	if err != nil {
		log.Fatalf("Failed to open db: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping db: %v", err)
	}

	return db
}
