package voting

import "github.com/egp/rcv-app/internal/models"

// Tabulate calculates the winner using Ranked Choice Voting logic.
func Tabulate(candidates []int, ballots []models.Ballot) (int, []models.RoundResult) {
	activeCandidates := make(map[int]bool)
	var rounds []models.RoundResult

	for _, id := range candidates {
		activeCandidates[id] = true
	}

	for r := 1; len(activeCandidates) > 1; r++ {
		counts := make(map[int]int)
		totalVotes := 0

		// 1. Count top preference for ballot among active candidates
		for _, b := range ballots {
			for _, candidateID := range b.Rankings {
				if activeCandidates[candidateID] {
					counts[candidateID]++
					totalVotes++
					break
				}
			}
		}

		// 2. Check for majority winner
		for id, count := range counts {
			if float64(count) > float64(totalVotes)/2 {
				rounds = append(rounds, models.RoundResult{
					RoundNumber: r,
					VoteCounts:  counts,
				})
				return id, rounds
			}
		}

		// 3. Find and eliminate the loser(s)
		minVotes := totalVotes + 1
		loserID := -1
		for id := range activeCandidates {
			if counts[id] < minVotes {
				minVotes = counts[id]
				loserID = id
			}
		}

		delete(activeCandidates, loserID)
		rounds = append(rounds, models.RoundResult{
			RoundNumber: r,
			VoteCounts:  counts,
			Eliminated:  loserID,
		})
	}

	// Return the last remaining candidate if no majority was reached earlier
	for id := range activeCandidates {
		return id, rounds
	}

	return -1, rounds
}
