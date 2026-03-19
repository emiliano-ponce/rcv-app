package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"sort"

	"github.com/egp/rcv-app/internal/models"
	"github.com/egp/rcv-app/internal/voting"
)

// generateKey produces a cryptographically random 8-character hex key.
func generateKey() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// render executes a named template, logging errors but never double-writing headers.
func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	if err := h.Tmpls.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %q: %v", name, err)
		// Only write error if headers haven't been sent yet.
		http.Error(w, "rendering error", http.StatusInternalServerError)
	}
}

// getPollByKey fetches a poll and its candidates by the share key.
func (h *Handler) getPollByKey(key string) (models.Poll, error) {
	var poll models.Poll
	err := h.DB.QueryRow(
		"SELECT id, key, title, description FROM polls WHERE key = ?", key,
	).Scan(&poll.ID, &poll.Key, &poll.Title, &poll.Description)
	if err != nil {
		return poll, err
	}

	rows, err := h.DB.Query(
		"SELECT id, name FROM candidates WHERE poll_id = ? ORDER BY display_order", poll.ID,
	)
	if err != nil {
		return poll, err
	}
	defer rows.Close()

	for rows.Next() {
		var c models.Candidate
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return poll, err
		}
		poll.Candidates = append(poll.Candidates, c)
	}
	return poll, rows.Err()
}

// buildResultsData fetches ballots and runs the tabulation for a poll.
func (h *Handler) buildResultsData(poll models.Poll) (resultsData, error) {
	data := resultsData{Poll: poll}

	// Build candidate name lookup.
	nameMap := make(map[int]string)
	var candidateIDs []int
	for _, c := range poll.Candidates {
		nameMap[c.ID] = c.Name
		candidateIDs = append(candidateIDs, c.ID)
	}

	// Count ballots.
	err := h.DB.QueryRow("SELECT COUNT(*) FROM ballots WHERE poll_id = ?", poll.ID).
		Scan(&data.BallotCount)
	if err != nil {
		return data, fmt.Errorf("count ballots: %w", err)
	}

	if data.BallotCount == 0 {
		return data, nil
	}
	data.HasBallots = true

	// Fetch ballot rankings.
	rows, err := h.DB.Query(`
		SELECT b.id, br.candidate_id
		FROM ballots b
		JOIN ballot_rankings br ON b.id = br.ballot_id
		WHERE b.poll_id = ?
		ORDER BY b.id, br.rank`, poll.ID)
	if err != nil {
		return data, fmt.Errorf("fetch rankings: %w", err)
	}
	defer rows.Close()

	ballotMap := make(map[int]*models.Ballot)
	var orderedIDs []int

	for rows.Next() {
		var bID, cID int
		if err := rows.Scan(&bID, &cID); err != nil {
			log.Printf("buildResultsData: scan row: %v", err)
			continue
		}
		if _, exists := ballotMap[bID]; !exists {
			ballotMap[bID] = &models.Ballot{ID: bID}
			orderedIDs = append(orderedIDs, bID)
		}
		ballotMap[bID].Rankings = append(ballotMap[bID].Rankings, cID)
	}
	if err := rows.Err(); err != nil {
		return data, fmt.Errorf("row iteration: %w", err)
	}

	var ballots []models.Ballot
	for _, id := range orderedIDs {
		ballots = append(ballots, *ballotMap[id])
	}

	winnerID, rounds := voting.Tabulate(candidateIDs, ballots)
	data.WinnerID = winnerID
	if name, ok := nameMap[winnerID]; ok {
		data.WinnerName = name
	}

	// Convert raw RoundResults into template-friendly roundViews.
	for _, r := range rounds {
		rv := roundView{
			RoundNumber:    r.RoundNumber,
			TotalVotes:     r.TotalVotes,
			EliminatedID:   r.EliminatedID,
			EliminatedName: nameMap[r.EliminatedID],
			HasEliminated:  r.HasEliminated,
			IsWinnerRound:  r.IsWinnerRound,
		}

		for id, votes := range r.VoteCounts {
			pct := 0
			if r.TotalVotes > 0 {
				pct = (votes * 100) / r.TotalVotes
			}
			rv.Candidates = append(rv.Candidates, candidateVote{
				ID:           id,
				Name:         nameMap[id],
				Votes:        votes,
				Percent:      pct,
				IsEliminated: r.HasEliminated && id == r.EliminatedID,
			})
		}

		// Sort by votes descending for a consistent bar chart.
		sort.Slice(rv.Candidates, func(i, j int) bool {
			return rv.Candidates[i].Votes > rv.Candidates[j].Votes
		})

		data.Rounds = append(data.Rounds, rv)
	}

	return data, nil
}
