package voting

import (
	"sort"

	"github.com/egp/rcv-app/internal/models"
)

// Tabulate calculates the winner using Ranked Choice Voting.
// Returns winnerID (-1 if undecidable) and a round-by-round breakdown.
//
// Tie-breaking when multiple candidates share the round minimum:
//  1. Borda Count across all ballots (fewest points eliminated first)
//  2. Lowest candidate ID if Borda scores are also equal
func Tabulate(candidates []int, ballots []models.Ballot) (int, []models.RoundResult) {
	// Single candidate wins immediately.
	if len(candidates) == 1 {
		return candidates[0], []models.RoundResult{}
	}

	activeCandidates := make(map[int]bool)
	var rounds []models.RoundResult
	for _, id := range candidates {
		activeCandidates[id] = true
	}

	for r := 1; len(activeCandidates) > 1; r++ {
		counts := make(map[int]int)
		totalVotes := 0

		// Count each ballot's top-ranked active candidate.
		for _, b := range ballots {
			for _, candidateID := range b.Rankings {
				if activeCandidates[candidateID] {
					counts[candidateID]++
					totalVotes++
					break
				}
			}
		}

		// Guard: all ballots exhausted — no majority possible.
		if totalVotes == 0 {
			break
		}

		// Check for a majority winner (strictly more than half).
		for id, count := range counts {
			if float64(count) > float64(totalVotes)/2 {
				rounds = append(rounds, models.RoundResult{
					RoundNumber:   r,
					VoteCounts:    counts,
					TotalVotes:    totalVotes,
					IsWinnerRound: true,
				})
				return id, rounds
			}
		}

		// Find the minimum vote count among active candidates.
		// Candidates with 0 votes (exhausted ballots) are included.
		minVotes := totalVotes + 1
		for id := range activeCandidates {
			if counts[id] < minVotes {
				minVotes = counts[id]
			}
		}

		// Collect all candidates tied at the minimum.
		var tied []int
		for id := range activeCandidates {
			if counts[id] == minVotes {
				tied = append(tied, id)
			}
		}

		loserID := breakTie(tied, activeCandidates, ballots)
		delete(activeCandidates, loserID)

		rounds = append(rounds, models.RoundResult{
			RoundNumber:   r,
			VoteCounts:    counts,
			TotalVotes:    totalVotes,
			EliminatedID:  loserID,
			HasEliminated: true,
		})
	}

	if len(rounds) == 0 {
		return -1, rounds
	}

	// Last candidate standing after all eliminations.
	for id := range activeCandidates {
		return id, rounds
	}

	return -1, rounds
}

// breakTie selects which candidate to eliminate from a tied group.
//
//  1. Compute Borda scores for the tied candidates using all ballots
//     (only active candidates contribute to the positional score).
//  2. Eliminate the candidate with the lowest Borda score.
//  3. If Borda scores are equal, eliminate the candidate with the lowest ID.
func breakTie(tied []int, activeCandidates map[int]bool, ballots []models.Ballot) int {
	if len(tied) == 1 {
		return tied[0]
	}

	bordaScores := computeBordaScores(activeCandidates, ballots)

	minBorda := -1
	for _, id := range tied {
		if minBorda == -1 || bordaScores[id] < minBorda {
			minBorda = bordaScores[id]
		}
	}

	// Collect candidates still tied after Borda.
	var bordaTied []int
	for _, id := range tied {
		if bordaScores[id] == minBorda {
			bordaTied = append(bordaTied, id)
		}
	}

	// Fall back to lowest ID if Borda didn't resolve the tie.
	sort.Ints(bordaTied)
	return bordaTied[0]
}

// computeBordaScores assigns each active candidate a score based on their
// position in every ballot. For n active candidates, a candidate ranked
// first earns n-1 points, second earns n-2, and so on. Unranked active
// candidates earn 0. Inactive candidates are skipped entirely.
func computeBordaScores(activeCandidates map[int]bool, ballots []models.Ballot) map[int]int {
	n := len(activeCandidates)
	scores := make(map[int]int, n)

	for id := range activeCandidates {
		scores[id] = 0
	}

	for _, b := range ballots {
		rank := 0 // position among active candidates seen so far on this ballot
		for _, id := range b.Rankings {
			if !activeCandidates[id] {
				continue
			}
			// Points awarded: n-1 for rank 0 (first active), n-2 for rank 1, etc.
			points := n - 1 - rank
			scores[id] += points
			rank++
		}
	}

	return scores
}
