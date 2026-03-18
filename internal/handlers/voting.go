package handlers

import (
	"database/sql"
	"html/template"
	"net/http"

	"github.com/egp/rcv-app/internal/models"
	"github.com/egp/rcv-app/internal/voting"
)

type Handler struct {
	DB    *sql.DB
	Tmpls *template.Template
}

func (h *Handler) TabulateHandler(w http.ResponseWriter, r *http.Request) {
	pollID := r.FormValue("poll_id")
	if pollID == "" {
		http.Error(w, "Missing poll_id", http.StatusBadRequest)
		return
	}

	candidateRows, err := h.DB.Query("SELECT id FROM candidates WHERE poll_id = ?", pollID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer candidateRows.Close()

	var candidateIDs []int
	for candidateRows.Next() {
		var id int
		if err := candidateRows.Scan(&id); err == nil {
			candidateIDs = append(candidateIDs, id)
		}
	}

	rows, err := h.DB.Query(`
		SELECT b.id, br.candidate_id 
		FROM ballots b 
		JOIN ballot_rankings br ON b.id = br.ballot_id 
		WHERE b.poll_id = ?
		ORDER BY b.id, br.rank`, pollID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	ballotMap := make(map[int]*models.Ballot)
	var orderedIDs []int

	for rows.Next() {
		var bID, cID int
		if err := rows.Scan(&bID, &cID); err != nil {
			continue
		}

		if _, exists := ballotMap[bID]; !exists {
			ballotMap[bID] = &models.Ballot{ID: bID}
			orderedIDs = append(orderedIDs, bID)
		}
		ballotMap[bID].Rankings = append(ballotMap[bID].Rankings, cID)
	}

	var ballots []models.Ballot
	for _, id := range orderedIDs {
		ballots = append(ballots, *ballotMap[id])
	}

	winnerID, rounds := voting.Tabulate(candidateIDs, ballots)

	data := map[string]interface{}{
		"Winner": winnerID,
		"Rounds": rounds,
	}

	err = h.Tmpls.ExecuteTemplate(w, "results-table", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
