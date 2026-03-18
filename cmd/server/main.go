package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/egp/rcv-app/internal/database"
	"github.com/egp/rcv-app/internal/handlers"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "file:dev.db"
		log.Println("DATABASE_URL not set, using local dev.db")
	}

	db := database.InitDB(dbURL)
	defer db.Close()

	tmpls := template.Must(template.ParseFiles("ui/html/index.html"))

	h := &handlers.Handler{
		DB:    db,
		Tmpls: tmpls,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpls.ExecuteTemplate(w, "index", nil)
	})

	http.HandleFunc("/tabulate", h.TabulateHandler)

	log.Println("Server starting at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
