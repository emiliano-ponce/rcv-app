package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/egp/rcv-app/internal/models"
	"github.com/egp/rcv-app/internal/voting"
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

// generateKey produces a cryptographically random 8-character hex key.
func generateKey() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func NewHandler(db *sql.DB) *Handler {
	cache, err := newTemplateCache()
	if err != nil {
		log.Fatal(err)
	}

	return &Handler{
		DB:            db,
		TemplateCache: cache,
	}
}

func newTemplateCache() (map[string]*template.Template, error) {
	cache := make(map[string]*template.Template)

	pages, err := filepath.Glob("ui/html/pages/*.html")
	if err != nil {
		return nil, err
	}

	// Discover partials dynamically — no more hardcoding
	partials, err := filepath.Glob("ui/html/partials/*.html")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)

		// Build the full file list: base + all partials + this page
		// Order matters: base first so it's the named template set root
		files := make([]string, 0, len(partials)+2)
		files = append(files, "ui/html/base.html")
		files = append(files, partials...)
		files = append(files, page)

		ts, err := template.New(name).Funcs(templateFuncs).ParseFiles(files...)
		if err != nil {
			return nil, fmt.Errorf("parsing template %q: %w", name, err)
		}

		cache[name] = ts
	}

	return cache, nil
}

// render executes the full base layout.
func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	ts, ok := h.TemplateCache[name+".html"]
	if !ok {
		log.Printf("template %q not found", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := ts.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("render %q: %v", name, err)
		// Note: can't change status code here if WriteHeader already called
	}
}

// renderFragment executes a named partial template without the base layout.
// page is the cache key (e.g. "results"), tmpl is the defined template name.
func (h *Handler) renderFragment(w http.ResponseWriter, page string, tmpl string, data any) {
	ts, ok := h.TemplateCache[page+".html"]
	if !ok {
		log.Printf("template %q not found for fragment %q", page, tmpl)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := ts.ExecuteTemplate(w, tmpl, data); err != nil {
		log.Printf("renderFragment %q/%q: %v", page, tmpl, err)
	}
}

// getPollByKey fetches a poll and its candidates by the share key.
func (h *Handler) getPollByKey(key string) (models.Poll, error) {
	var poll models.Poll
	err := h.DB.QueryRow(`SELECT id, key, title, description, created_at, closed_at FROM polls WHERE key = ?`, key).
		Scan(&poll.ID, &poll.Key, &poll.Title, &poll.Description, &poll.CreatedAt, &poll.ClosedAt)
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
