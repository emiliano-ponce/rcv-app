package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/egp/rcv-app/internal/database"
	"github.com/egp/rcv-app/internal/handlers"
	"github.com/egp/rcv-app/internal/security"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file if it exists (for local development).
	if os.Getenv("DATABASE_URL") == "" {
		if err := godotenv.Load(); err != nil {
			log.Printf("No .env file found or failed to load: %v", err)
		}
	}

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

	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	isDevEnv := appEnv == "dev" || appEnv == "development" || appEnv == "local"

	turnstileSiteKey := strings.TrimSpace(os.Getenv("CF_TURNSTILE_SITE_KEY"))
	turnstileSecret := strings.TrimSpace(os.Getenv("CF_TURNSTILE_SECRET_KEY"))
	defaultDisableTurnstile := isDevEnv && (turnstileSiteKey == "" || turnstileSecret == "")
	h.DisableTurnstile = parseBoolEnv("DISABLE_TURNSTILE_VERIFY", defaultDisableTurnstile)
	if h.DisableTurnstile {
		h.TurnstileKey = ""
		h.Turnstile = nil
		log.Println("DISABLE_TURNSTILE_VERIFY enabled: vote turnstile verification is bypassed")
	} else {
		h.TurnstileKey = turnstileSiteKey
		h.Turnstile = security.NewCloudflareTurnstileVerifier(turnstileSecret, &http.Client{Timeout: 5 * time.Second})
	}

	defaultAllowDevMultiVote := isDevEnv
	h.AllowDevMultiVote = parseBoolEnv("ALLOW_DEV_MULTI_VOTE", defaultAllowDevMultiVote)

	if h.AllowDevMultiVote {
		log.Println("ALLOW_DEV_MULTI_VOTE enabled: vote cookie guard and vote rate limit are disabled")
	}

	createRatePerMinute := parseIntEnv("RCV_CREATE_RATE_LIMIT_PER_MIN", 5)
	voteRatePerMinute := parseIntEnv("RCV_VOTE_RATE_LIMIT_PER_MIN", 15)
	createLimiter := security.NewRateLimiter(createRatePerMinute, time.Minute)
	voteLimiter := security.NewRateLimiter(voteRatePerMinute, time.Minute)

	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("ui/static/"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	mux.HandleFunc("GET /", h.HomeHandler)
	mux.HandleFunc("POST /find", h.FindPollHandler)
	mux.HandleFunc("POST /polls", security.WrapWithRateLimit(createLimiter, h.CreatePollHandler))
	mux.HandleFunc("GET /polls/{key}", h.VoteHandler)
	voteSubmitHandler := h.SubmitBallotHandler
	if !h.AllowDevMultiVote {
		voteSubmitHandler = security.WrapWithRateLimit(voteLimiter, voteSubmitHandler)
	}
	mux.HandleFunc("POST /polls/{key}/vote", voteSubmitHandler)
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

func parseBoolEnv(name string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func parseIntEnv(name string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
