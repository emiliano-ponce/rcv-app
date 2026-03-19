package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/egp/rcv-app/internal/database"
	"github.com/egp/rcv-app/internal/handlers"
)

// templateFuncs provides helpers available in all templates.
var templateFuncs = template.FuncMap{
	// seq returns [1, 2, ..., n] for ranging in templates.
	"seq": func(n int) []int {
		s := make([]int, n)
		for i := range s {
			s[i] = i + 1
		}
		return s
	},
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	// percent returns the integer percentage of count out of total (0–100).
	// Safe: returns 0 when total is 0.
	"percent": func(count, total int) int {
		if total == 0 {
			return 0
		}
		return (count * 100) / total
	},
}

func main() {
	// 1. Database URL — defaults to local file for dev.
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "file:dev.db"
		log.Println("DATABASE_URL not set, using local dev.db")
	}

	// 2. Initialize database (applies schema migrations automatically).
	db, err := database.InitDB(dbURL)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}
	defer db.Close()

	// 3. Parse all templates from ui/html/. FuncMap must be set before parsing.
	tmpls := template.Must(
		template.New("").Funcs(templateFuncs).ParseGlob("ui/html/*.html"),
	)

	h := &handlers.Handler{
		DB:    db,
		Tmpls: tmpls,
	}

	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("ui/static/"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("GET /", h.HomeHandler)
	mux.HandleFunc("POST /find", h.FindPollHandler)
	mux.HandleFunc("POST /polls", h.CreatePollHandler)
	mux.HandleFunc("GET /polls/{key}/created", h.PollCreatedHandler)
	mux.HandleFunc("GET /polls/{key}", h.VoteHandler)
	mux.HandleFunc("POST /polls/{key}/vote", h.SubmitBallotHandler)
	mux.HandleFunc("GET /polls/{key}/thanks", h.ThanksHandler)
	mux.HandleFunc("GET /polls/{key}/results", h.ResultsHandler)
	mux.HandleFunc("GET /polls/{key}/results/fragment", h.ResultsFragmentHandler)

	log.Println("Server starting at :8080")
	log.Fatal(http.ListenAndServe("localhost:8080", mux))
}
