package handlers

import (
	"database/sql"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/egp/rcv-app/internal/security"
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

	rand.Shuffle(len(poll.Candidates), func(i, j int) {
		poll.Candidates[i], poll.Candidates[j] = poll.Candidates[j], poll.Candidates[i]
	})

	h.render(w, "vote", voteData{Poll: poll, TurnstileKey: h.TurnstileKey})
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

	if !h.AllowDevMultiVote {
		if _, err := r.Cookie(voteCookieName(key)); err == nil {
			h.render(w, "vote", voteData{
				Poll:         poll,
				Error:        "This browser already submitted a vote for this poll.",
				TurnstileKey: h.TurnstileKey,
			})
			return
		}
	}

	if !h.DisableTurnstile {
		token := strings.TrimSpace(r.FormValue("cf-turnstile-response"))
		if token == "" {
			h.render(w, "vote", voteData{Poll: poll, Error: "Please complete the verification challenge.", TurnstileKey: h.TurnstileKey})
			return
		}

		if h.Turnstile == nil {
			log.Printf("SubmitBallotHandler: turnstile verifier is not configured")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		ok, err := h.Turnstile.Verify(r.Context(), token, security.ClientIP(r))
		if err != nil {
			log.Printf("SubmitBallotHandler: turnstile verify: %v", err)
			h.render(w, "vote", voteData{Poll: poll, Error: "Could not verify challenge. Please try again.", TurnstileKey: h.TurnstileKey})
			return
		}
		if !ok {
			h.render(w, "vote", voteData{Poll: poll, Error: "Verification failed. Please try again.", TurnstileKey: h.TurnstileKey})
			return
		}
	}

	// Parse the ordered ranking from the form: "rankings" is a
	// comma-separated list of candidate IDs in the voter's preferred order.
	rawRankings := strings.TrimSpace(r.FormValue("rankings"))
	if rawRankings == "" {
		h.render(w, "vote", voteData{Poll: poll, Error: "Please rank at least one candidate before submitting.", TurnstileKey: h.TurnstileKey})
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
			h.render(w, "vote", voteData{Poll: poll, Error: "Invalid ballot data. Please try again.", TurnstileKey: h.TurnstileKey})
			return
		}
		seen[id] = true
		rankings = append(rankings, id)
	}

	if len(rankings) == 0 {
		h.render(w, "vote", voteData{Poll: poll, Error: "Please rank at least one candidate before submitting.", TurnstileKey: h.TurnstileKey})
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

	if !h.AllowDevMultiVote {
		http.SetCookie(w, &http.Cookie{
			Name:     voteCookieName(key),
			Value:    "voted",
			Path:     "/polls/" + key,
			MaxAge:   int((30 * 24 * time.Hour).Seconds()),
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
		})
	}

	http.Redirect(w, r, "/polls/"+key+"/thanks", http.StatusSeeOther)
}

func voteCookieName(key string) string {
	return "rcv_vote_" + key
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
