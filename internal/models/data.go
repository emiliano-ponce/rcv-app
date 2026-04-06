package models

import "database/sql"

type Poll struct {
	ID          int
	Key         string
	Title       string
	Description string
	CreatedAt   string
	Candidates  []Candidate
	ClosedAt    sql.NullTime
}

type Candidate struct {
	ID           int
	PollID       int
	Name         string
	DisplayOrder int
}

type Ballot struct {
	ID       int
	Rankings []int
}

type RoundResult struct {
	RoundNumber   int
	VoteCounts    map[int]int
	TotalVotes    int // sum of all counts this round (excludes exhausted ballots)
	EliminatedID  int
	HasEliminated bool
	IsWinnerRound bool
}
