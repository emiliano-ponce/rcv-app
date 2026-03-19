package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// VoteHandler serves GET /polls/{key} — the voting page.
func (h *Handler) VoteHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	poll, err := h.getPollByKey(key)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("VoteHandler: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.render(w, "vote", voteData{Poll: poll})
}

// SubmitBallotHandler serves POST /polls/{key}/vote — records a ranked ballot.
func (h *Handler) SubmitBallotHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	poll, err := h.getPollByKey(key)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("SubmitBallotHandler: get poll: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Parse the ordered ranking from the form: "rankings" is a
	// comma-separated list of candidate IDs in the voter's preferred order.
	rawRankings := strings.TrimSpace(r.FormValue("rankings"))
	if rawRankings == "" {
		h.render(w, "vote", voteData{Poll: poll, Error: "Please rank at least one candidate before submitting."})
		return
	}

	parts := strings.Split(rawRankings, ",")

	// Validate candidate IDs belong to this poll.
	validIDs := make(map[int]bool)
	for _, c := range poll.Candidates {
		validIDs[c.ID] = true
	}

	var rankings []int
	seen := make(map[int]bool)
	for _, p := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || !validIDs[id] || seen[id] {
			h.render(w, "vote", voteData{Poll: poll, Error: "Invalid ballot data. Please try again."})
			return
		}
		seen[id] = true
		rankings = append(rankings, id)
	}

	if len(rankings) == 0 {
		h.render(w, "vote", voteData{Poll: poll, Error: "Please rank at least one candidate before submitting."})
		return
	}

	// Insert ballot.
	res, err := h.DB.Exec("INSERT INTO ballots (poll_id) VALUES (?)", poll.ID)
	if err != nil {
		log.Printf("SubmitBallotHandler: insert ballot: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	ballotID, err := res.LastInsertId()
	if err != nil {
		log.Printf("SubmitBallotHandler: last insert id: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Insert rankings.
	for rank, candidateID := range rankings {
		_, err := h.DB.Exec(
			"INSERT INTO ballot_rankings (ballot_id, candidate_id, rank) VALUES (?, ?, ?)",
			ballotID, candidateID, rank+1,
		)
		if err != nil {
			log.Printf("SubmitBallotHandler: insert ranking: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/polls/"+key+"/thanks", http.StatusSeeOther)
}

// ThanksHandler serves GET /polls/{key}/thanks — post-submission confirmation.
func (h *Handler) ThanksHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	poll, err := h.getPollByKey(key)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("ThanksHandler: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h.render(w, "thanks", thanksData{Poll: poll})
}
