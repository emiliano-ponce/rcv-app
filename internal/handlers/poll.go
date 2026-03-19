package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// HomeHandler serves GET / — the landing page.
func (h *Handler) HomeHandler(w http.ResponseWriter, r *http.Request) {
	h.render(w, "home", homeData{})
}

// FindPollHandler serves POST /find — redirects to a poll by key.
func (h *Handler) FindPollHandler(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.FormValue("key"))
	if key == "" {
		h.render(w, "home", homeData{Error: "Please enter a poll key."})
		return
	}

	var exists int
	err := h.DB.QueryRow("SELECT COUNT(*) FROM polls WHERE key = ?", key).Scan(&exists)
	if err != nil || exists == 0 {
		h.render(w, "home", homeData{Error: "Poll not found. Check the key and try again."})
		return
	}

	http.Redirect(w, r, "/polls/"+key, http.StatusSeeOther)
}

// CreatePollHandler serves POST /polls — creates a new poll and its candidates.
func (h *Handler) CreatePollHandler(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))

	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	// Collect non-empty candidate names.
	var candidateNames []string
	for i := 1; i <= 10; i++ {
		name := strings.TrimSpace(r.FormValue(fmt.Sprintf("candidate_%d", i)))
		if name != "" {
			candidateNames = append(candidateNames, name)
		}
	}
	if len(candidateNames) < 3 {
		http.Error(w, "at least 3 candidates are required", http.StatusBadRequest)
		return
	}

	key, err := generateKey()
	if err != nil {
		log.Printf("CreatePollHandler: generate key: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Insert poll.
	res, err := h.DB.Exec(
		"INSERT INTO polls (key, title, description) VALUES (?, ?, ?)",
		key, title, description,
	)
	if err != nil {
		log.Printf("CreatePollHandler: insert poll: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	pollID, err := res.LastInsertId()
	if err != nil {
		log.Printf("CreatePollHandler: last insert id: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Insert candidates.
	for i, name := range candidateNames {
		_, err := h.DB.Exec(
			"INSERT INTO candidates (poll_id, name, display_order) VALUES (?, ?, ?)",
			pollID, name, i+1,
		)
		if err != nil {
			log.Printf("CreatePollHandler: insert candidate %q: %v", name, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/polls/"+key+"/created", http.StatusSeeOther)
}

// PollCreatedHandler serves GET /polls/{key}/created — the share screen after creation.
func (h *Handler) PollCreatedHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	poll, err := h.getPollByKey(key)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("PollCreatedHandler: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := scheme + "://" + r.Host

	h.render(w, "poll-created", pollCreatedData{Poll: poll, BaseURL: baseURL})
}
