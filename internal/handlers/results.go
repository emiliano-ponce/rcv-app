package handlers

import (
	"database/sql"
	"log"
	"net/http"
)

// ResultsHandler serves GET /polls/{key}/results — the full results page.
func (h *Handler) ResultsHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	poll, err := h.getPollByKey(key)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("ResultsHandler: get poll: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	data, err := h.buildResultsData(poll)
	if err != nil {
		log.Printf("ResultsHandler: build results: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	data.IsOwner = isPollOwner(r, key)
	data.HasVoted = hasVoted(r, key)

	if _, err := r.Cookie(toastCookieName(key)); err == nil {
		data.ShowVotedToast = true
		http.SetCookie(w, &http.Cookie{
			Name:     toastCookieName(key),
			Value:    "",
			Path:     "/polls/" + key,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
		})
	}

	h.render(w, "results", data)
}

// ResultsFragmentHandler serves GET /polls/{key}/results/fragment — the HTMX polling target.
func (h *Handler) ResultsFragmentHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	poll, err := h.getPollByKey(key)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("ResultsFragmentHandler: get poll: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	data, err := h.buildResultsData(poll)
	if err != nil {
		log.Printf("ResultsFragmentHandler: build results: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.renderFragment(w, "results", "results-fragment", data)
}
