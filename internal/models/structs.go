package models

type Candidate struct {
	ID   int
	Name string
}

type Ballot struct {
	ID       int
	Rankings []int
}

type Poll struct {
	ID          string
	Title       string
	Description string
}

type RoundResult struct {
	RoundNumber int
	VoteCounts  map[int]int
	Eliminated  int
}
