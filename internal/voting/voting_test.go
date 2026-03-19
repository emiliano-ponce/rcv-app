package voting

import (
	"testing"

	"github.com/egp/rcv-app/internal/models"
)

func mkBallots(rankings ...[]int) []models.Ballot {
	ballots := make([]models.Ballot, len(rankings))
	for i, r := range rankings {
		ballots[i] = models.Ballot{ID: i + 1, Rankings: r}
	}
	return ballots
}

func TestTabulate(t *testing.T) {
	tests := []struct {
		name              string
		candidates        []int
		ballots           []models.Ballot
		wantWinner        int
		wantRounds        int
		wantEliminatedSeq []int // eliminated candidate per round, in order
	}{
		{
			// Candidate 1 clears >50% in round 1 — no eliminations needed.
			name:       "immediate majority round 1",
			candidates: []int{1, 2, 3},
			ballots: mkBallots(
				[]int{1, 2, 3},
				[]int{1, 3, 2},
				[]int{1, 2, 3},
				[]int{2, 1, 3},
				[]int{3, 2, 1},
			),
			wantWinner:        1,
			wantRounds:        1,
			wantEliminatedSeq: []int{},
		},
		{
			// Round 1: 1→3, 2→2, 3→1. Eliminate 3.
			// Round 2: 1→4, 2→2. Candidate 1 wins.
			name:       "winner in round 2",
			candidates: []int{1, 2, 3},
			ballots: mkBallots(
				[]int{1, 2, 3},
				[]int{1, 2, 3},
				[]int{1, 2, 3},
				[]int{2, 1, 3},
				[]int{2, 1, 3},
				[]int{3, 1, 2},
			),
			wantWinner:        1,
			wantRounds:        2,
			wantEliminatedSeq: []int{3},
		},
		{
			// Round 1: 1→3, 2→3, 3→2, 4→1. Eliminate 4.
			// Round 2: 1→4, 2→3, 3→2.         Eliminate 3.
			// Round 3: 1→4, 2→5.               Candidate 2 wins.
			name:       "winner in round 3",
			candidates: []int{1, 2, 3, 4},
			ballots: mkBallots(
				[]int{1, 2, 3, 4},
				[]int{1, 2, 3, 4},
				[]int{1, 2, 3, 4},
				[]int{2, 3, 1, 4},
				[]int{2, 3, 1, 4},
				[]int{2, 3, 1, 4},
				[]int{3, 2, 1, 4},
				[]int{3, 2, 1, 4},
				[]int{4, 1, 2, 3},
			),
			wantWinner:        2,
			wantRounds:        3,
			wantEliminatedSeq: []int{4, 3},
		},
		{
			// Only one candidate — loop body never executes.
			name:       "single candidate wins immediately",
			candidates: []int{42},
			ballots: mkBallots(
				[]int{42},
				[]int{42},
			),
			wantWinner: 42,
			wantRounds: 0,
		},
		{
			// Two candidates, one ballot each — 50% is not a majority (needs strictly >50%).
			// Candidate 2 has 0 votes so it is eliminated; candidate 1 wins as last standing.
			name:       "two candidates strict majority check",
			candidates: []int{1, 2},
			ballots: mkBallots(
				[]int{1, 2},
			),
			wantWinner:        1,
			wantRounds:        1,
			wantEliminatedSeq: []int{2},
		},
		{
			// Ballots tied for votes, BordaCount equal, lowest IDs elminated.
			name:       "three-way tie resolves to highest ID via successive lowest-ID elimination",
			candidates: []int{1, 2, 3},
			ballots: mkBallots(
				[]int{3},
				[]int{1},
				[]int{2},
			),
			wantWinner:        3,
			wantRounds:        2,
			wantEliminatedSeq: []int{1, 2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotWinner, gotRounds := Tabulate(tc.candidates, tc.ballots)

			if gotWinner != tc.wantWinner {
				t.Errorf("winner: got %d, want %d", gotWinner, tc.wantWinner)
			}
			if len(gotRounds) != tc.wantRounds {
				t.Errorf("round count: got %d, want %d", len(gotRounds), tc.wantRounds)
			}

			// Verify elimination sequence when specified.
			elimIdx := 0
			for _, r := range gotRounds {
				if r.HasEliminated {
					if elimIdx >= len(tc.wantEliminatedSeq) {
						t.Errorf("unexpected elimination of %d in round %d", r.EliminatedID, r.RoundNumber)
						continue
					}
					if r.EliminatedID != tc.wantEliminatedSeq[elimIdx] {
						t.Errorf("round %d: eliminated %d, want %d",
							r.RoundNumber, r.EliminatedID, tc.wantEliminatedSeq[elimIdx])
					}
					elimIdx++
				}
			}
		})
	}
}

func TestTabulate_RoundStructure(t *testing.T) {
	// Validates fields on RoundResult beyond just winner/loser identity.
	candidates := []int{1, 2, 3}
	ballots := mkBallots(
		[]int{1, 2, 3},
		[]int{1, 2, 3},
		[]int{1, 2, 3},
		[]int{2, 1, 3},
		[]int{2, 1, 3},
		[]int{3, 1, 2},
	)

	_, rounds := Tabulate(candidates, ballots)

	if len(rounds) != 2 {
		t.Fatalf("expected 2 rounds, got %d", len(rounds))
	}

	r1 := rounds[0]
	if r1.RoundNumber != 1 {
		t.Errorf("round 1 number: got %d", r1.RoundNumber)
	}
	if r1.TotalVotes != 6 {
		t.Errorf("round 1 total votes: got %d, want 6", r1.TotalVotes)
	}
	if r1.IsWinnerRound {
		t.Error("round 1 should not be marked as winner round")
	}
	if !r1.HasEliminated || r1.EliminatedID != 3 {
		t.Errorf("round 1: expected elimination of 3, got eliminated=%v id=%d",
			r1.HasEliminated, r1.EliminatedID)
	}

	r2 := rounds[1]
	if !r2.IsWinnerRound {
		t.Error("round 2 should be marked as winner round")
	}
	if r2.HasEliminated {
		t.Error("winner round should not also mark an elimination")
	}
	if r2.VoteCounts[1] != 4 {
		t.Errorf("round 2 candidate 1 votes: got %d, want 4", r2.VoteCounts[1])
	}
}

func TestTabulate_VoteTransfer(t *testing.T) {
	// Isolated test: confirms that after a candidate is eliminated, their
	// voters' second choices are counted in the next round.
	candidates := []int{1, 2, 3}
	ballots := mkBallots(
		[]int{1, 2, 3},
		[]int{1, 2, 3},
		[]int{1, 2, 3},
		[]int{2, 1, 3},
		[]int{3, 2, 1},
		[]int{3, 1, 2},
		[]int{3, 2, 1},
	)

	winner, rounds := Tabulate(candidates, ballots)
	if winner != 1 {
		t.Errorf("winner: got %d, want 1", winner)
	}

	r2 := rounds[1]
	if r2.VoteCounts[1] != 4 {
		t.Errorf("round 2 candidate 1 votes (after transfer): got %d, want 4",
			r2.VoteCounts[2])
	}
}

func TestTabulate_NoCandidates(t *testing.T) {
	winner, rounds := Tabulate([]int{}, []models.Ballot{})
	if winner != -1 {
		t.Errorf("expected -1 for no candidates, got %d", winner)
	}
	if len(rounds) != 0 {
		t.Errorf("expected 0 rounds, got %d", len(rounds))
	}
}

func TestTabulate_BordaResolvesToNonLowestID(t *testing.T) {
	// Candidates 1, 2, 3.
	//
	// Round 1 RCV:  1→2, 2→2, 3→3  →  no majority, tied losers: {1, 2}
	// Without Borda, lowest-ID rule would eliminate 1.
	//
	// Borda scores (3 active → 1st=2pts, 2nd=1pt, 3rd=0pts):
	//   ballot [3,1,2]×3:  3+=6, 1+=3, 2+=0
	//   ballot [1,3,2]×2:  1+=4, 3+=2, 2+=0
	//   ballot [2,1,3]×1:  2+=2, 1+=1, 3+=0
	//   ballot [2,3,1]×1:  2+=2, 3+=1, 1+=0
	//   totals → 1:8, 2:4, 3:9
	//
	// Borda minimum among tied {1,2} is candidate 2 (score 4).
	// Borda eliminates 2 — not 1 — which contradicts the lowest-ID rule.
	//
	// Round 2: active {1, 3}
	//   [3,1,2]×3 → 3,  [1,3,2]×2 → 1,  [2,1,3]×1 → 1,  [2,3,1]×1 → 3
	//   1→3, 3→4, total=7  →  3 wins (4 > 3.5)
	candidates := []int{1, 2, 3}
	ballots := mkBallots(
		[]int{3, 1, 2},
		[]int{3, 1, 2},
		[]int{3, 1, 2},
		[]int{1, 3, 2},
		[]int{1, 3, 2},
		[]int{2, 1, 3},
		[]int{2, 3, 1},
	)

	winner, rounds := Tabulate(candidates, ballots)

	if winner != 3 {
		t.Errorf("winner: got %d, want 3", winner)
	}
	if len(rounds) != 2 {
		t.Fatalf("round count: got %d, want 2", len(rounds))
	}

	r1 := rounds[0]
	if r1.EliminatedID != 2 {
		// If this prints 1, the Borda tiebreaker isn't firing — it fell
		// straight through to lowest-ID.
		t.Errorf("round 1 eliminated: got %d, want 2 (Borda should override lowest-ID here)", r1.EliminatedID)
	}
}

func TestTabulate_BordaAlsoTiedFallsBackToLowestID(t *testing.T) {
	// Candidates 1, 2, 3 with a perfectly symmetric ballot set.
	// Every tiebreaker except lowest-ID is exhausted.
	//
	// Round 1 RCV:  1→2, 2→2, 3→2  →  three-way tie at minimum
	//
	// Borda scores (3 active → 1st=2pts, 2nd=1pt, 3rd=0pts):
	//   ballot [1,2,3]×2:  1+=4, 2+=2, 3+=0
	//   ballot [2,3,1]×2:  2+=4, 3+=2, 1+=0
	//   ballot [3,1,2]×2:  3+=4, 1+=2, 2+=0
	//   totals → 1:6, 2:6, 3:6  (perfectly symmetric — Borda cannot differentiate)
	//
	// Lowest-ID fallback eliminates 1.
	//
	// Round 2: active {2, 3}
	//   [1,2,3]×2 → 2,  [2,3,1]×2 → 2,  [3,1,2]×2 → 3
	//   2→4, 3→2, total=6  →  2 wins (4 > 3)
	candidates := []int{1, 2, 3}
	ballots := mkBallots(
		[]int{1, 2, 3},
		[]int{1, 2, 3},
		[]int{2, 3, 1},
		[]int{2, 3, 1},
		[]int{3, 1, 2},
		[]int{3, 1, 2},
	)

	winner, rounds := Tabulate(candidates, ballots)

	if winner != 2 {
		t.Errorf("winner: got %d, want 2", winner)
	}
	if len(rounds) != 2 {
		t.Fatalf("round count: got %d, want 2", len(rounds))
	}

	r1 := rounds[0]
	if r1.EliminatedID != 1 {
		t.Errorf("round 1 eliminated: got %d, want 1 (lowest-ID fallback after Borda tie)", r1.EliminatedID)
	}
}
