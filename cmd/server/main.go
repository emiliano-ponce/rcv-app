package main

import (
	"log"
	"net/http"
	"os"

	"github.com/egp/rcv-app/internal/database"
	"github.com/egp/rcv-app/internal/handlers"
)

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

	h := handlers.NewHandler(db)

	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("ui/static/"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("GET /", h.HomeHandler)
	mux.HandleFunc("POST /find", h.FindPollHandler)
	mux.HandleFunc("POST /polls", h.CreatePollHandler)
	mux.HandleFunc("GET /polls/{key}", h.VoteHandler)
	mux.HandleFunc("POST /polls/{key}/vote", h.SubmitBallotHandler)
	mux.HandleFunc("GET /polls/{key}/thanks", h.ThanksHandler)
	mux.HandleFunc("GET /polls/{key}/results", h.ResultsHandler)
	mux.HandleFunc("GET /polls/{key}/results/fragment", h.ResultsFragmentHandler)

	mux.HandleFunc("GET /polls/{key}/manage", h.PollManageHandler)
	mux.HandleFunc("PATCH /polls/{key}", h.PollManageMetaHandler)
	mux.HandleFunc("DELETE /polls/{key}", h.PollManageDeleteHandler)
	mux.HandleFunc("POST /polls/{key}/candidates", h.PollManageAddCandidateHandler)
	mux.HandleFunc("PATCH /polls/{key}/candidates/{id}", h.PollManageUpdateCandidateHandler)
	mux.HandleFunc("DELETE /polls/{key}/candidates/{id}", h.PollManageDeleteCandidateHandler)
	mux.HandleFunc("POST /polls/{key}/close", h.PollManageCloseHandler)

	log.Println("Server starting at :8080")
	log.Fatal(http.ListenAndServe("localhost:8080", mux))
}
