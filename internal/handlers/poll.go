package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
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

	http.Redirect(w, r, "/polls/"+key+"/manage", http.StatusSeeOther)
}

// PollManageHandler serves GET /polls/{key}/manage
func (h *Handler) PollManageHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	poll, err := h.getPollByKey(key)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("PollManageHandler: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := scheme + "://" + r.Host

	h.render(w, "manage-poll", pollCreatedData{Poll: poll, BaseURL: baseURL})
}

// PollManageMetaHandler serves PATCH /polls/{key}
// Updates title and/or description.
func (h *Handler) PollManageMetaHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	if body.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	res, err := h.DB.Exec(
		`UPDATE polls SET title = ?, description = ? WHERE key = ? AND closed_at IS NULL`,
		body.Title, strings.TrimSpace(body.Description), key,
	)
	if err != nil {
		log.Printf("PollManageMetaHandler: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Either not found or already closed.
		http.Error(w, "poll not found or is closed", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PollManageAddCandidateHandler serves POST /polls/{key}/candidates
// Adds a new candidate to the poll.
func (h *Handler) PollManageAddCandidateHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	// Resolve poll ID and check not closed.
	var pollID int64
	var closedAt sql.NullTime
	err := h.DB.QueryRow(`SELECT id, closed_at FROM polls WHERE key = ?`, key).Scan(&pollID, &closedAt)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("PollManageAddCandidateHandler: lookup: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if closedAt.Valid {
		http.Error(w, "poll is closed", http.StatusForbidden)
		return
	}

	// Find the next display_order.
	var maxOrder int
	_ = h.DB.QueryRow(`SELECT COALESCE(MAX(display_order), 0) FROM candidates WHERE poll_id = ?`, pollID).Scan(&maxOrder)

	var candidateID int64
	res, err := h.DB.Exec(
		`INSERT INTO candidates (poll_id, name, display_order) VALUES (?, ?, ?)`,
		pollID, body.Name, maxOrder+1,
	)
	if err != nil {
		log.Printf("PollManageAddCandidateHandler: insert: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	candidateID, _ = res.LastInsertId()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": candidateID, "name": body.Name})
}

// PollManageUpdateCandidateHandler serves PATCH /polls/{key}/candidates/{id}
// Renames a candidate.
func (h *Handler) PollManageUpdateCandidateHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	candidateID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid candidate id", http.StatusBadRequest)
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	res, err := h.DB.Exec(`
		UPDATE candidates SET name = ?
		WHERE id = ? AND poll_id = (SELECT id FROM polls WHERE key = ? AND closed_at IS NULL)
	`, body.Name, candidateID, key)
	if err != nil {
		log.Printf("PollManageUpdateCandidateHandler: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "candidate not found or poll is closed", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PollManageDeleteCandidateHandler serves DELETE /polls/{key}/candidates/{id}
// Removes a candidate (only when ≥ 3 would remain).
func (h *Handler) PollManageDeleteCandidateHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	candidateID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid candidate id", http.StatusBadRequest)
		return
	}

	// Count remaining candidates first.
	var count int
	err = h.DB.QueryRow(`
		SELECT COUNT(*) FROM candidates
		WHERE poll_id = (SELECT id FROM polls WHERE key = ? AND closed_at IS NULL)
	`, key).Scan(&count)
	if err != nil {
		log.Printf("PollManageDeleteCandidateHandler: count: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if count <= 3 {
		http.Error(w, "polls must have at least 3 candidates", http.StatusConflict)
		return
	}

	res, err := h.DB.Exec(`
		DELETE FROM candidates
		WHERE id = ? AND poll_id = (SELECT id FROM polls WHERE key = ? AND closed_at IS NULL)
	`, candidateID, key)
	if err != nil {
		log.Printf("PollManageDeleteCandidateHandler: delete: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "candidate not found or poll is closed", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PollManageCloseHandler serves POST /polls/{key}/close
// Closes the poll, preventing any further votes.
func (h *Handler) PollManageCloseHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	res, err := h.DB.Exec(
		`UPDATE polls SET closed_at = ? WHERE key = ? AND closed_at IS NULL`,
		time.Now().UTC(), key,
	)
	if err != nil {
		log.Printf("PollManageCloseHandler: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "poll not found or already closed", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
