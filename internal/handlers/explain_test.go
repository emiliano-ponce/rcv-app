package handlers

import (
	"testing"

	"github.com/egp/rcv-app/internal/models"
	"github.com/egp/rcv-app/internal/voting"
)

func tieBallots(extraForC int) []models.Ballot {
	// A gets 3 first-choice votes; B, C, D tie at 1 each (unless extraForC > 0).
	var b []models.Ballot
	id := 1
	add := func(r ...int) { b = append(b, models.Ballot{ID: id, Rankings: r}); id++ }
	add(1, 2, 3, 4)
	add(1, 3, 2, 4)
	add(1, 4, 3, 2)
	add(2, 3, 1, 4)
	add(3, 2, 1, 4)
	add(4, 3, 2, 1)
	for i := 0; i < extraForC; i++ {
		add(3, 1, 2, 4)
	}
	return b
}

func explanations(ballots []models.Ballot) []string {
	names := map[int]string{1: "Alice", 2: "Bob", 3: "Carol", 4: "Dave"}
	_, rounds := voting.Tabulate([]int{1, 2, 3, 4}, ballots)
	out := make([]string, len(rounds))
	for i, r := range rounds {
		out[i] = explainRound(r, names)
	}
	return out
}

func TestExplainRound_IdenticalInputsIdenticalOutput(t *testing.T) {
	want := explanations(tieBallots(0))
	for i := 0; i < 200; i++ {
		got := explanations(tieBallots(0))
		if len(got) != len(want) {
			t.Fatalf("run %d: round count %d, want %d", i, len(got), len(want))
		}
		for j := range want {
			if got[j] != want[j] {
				t.Fatalf("run %d round %d:\n got %q\nwant %q", i, j+1, got[j], want[j])
			}
		}
	}
}

func TestExplainRound_TiedOrderIndependent(t *testing.T) {
	names := map[int]string{2: "Bob", 3: "Carol", 4: "Dave"}
	base := models.RoundResult{
		RoundNumber: 1, VoteCounts: map[int]int{2: 1, 3: 1, 4: 1}, TotalVotes: 3,
		EliminatedID: 2, HasEliminated: true, TiebreakMethod: "borda",
		BordaScores: map[int]int{2: 1, 3: 3, 4: 2},
	}
	a, b := base, base
	a.TiedCandidateIDs = []int{4, 2, 3}
	b.TiedCandidateIDs = []int{3, 4, 2}
	if ea, eb := explainRound(a, names), explainRound(b, names); ea != eb {
		t.Fatalf("order changed explanation:\n%q\n%q", ea, eb)
	}
}

func TestExplainRound_ChangedVotesChangeExplanation(t *testing.T) {
	before, after := explanations(tieBallots(0)), explanations(tieBallots(1))
	if before[0] == after[0] {
		t.Fatalf("expected round 1 explanation to change when votes change, got %q", before[0])
	}
}
