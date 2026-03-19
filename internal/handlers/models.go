package handlers

import (
	"database/sql"
	"html/template"

	"github.com/egp/rcv-app/internal/models"
)

type Handler struct {
	DB    *sql.DB
	Tmpls *template.Template
}

type homeData struct {
	Error string
}

type pollCreatedData struct {
	Poll    models.Poll
	BaseURL string
}

type voteData struct {
	Poll  models.Poll
	Error string
}

type thanksData struct {
	Poll models.Poll
}

// candidateVote pairs a candidate name with their vote count for a round.
type candidateVote struct {
	ID           int
	Name         string
	Votes        int
	Percent      int
	IsEliminated bool
}

// roundView is the template-friendly representation of one RCV round.
type roundView struct {
	RoundNumber    int
	Candidates     []candidateVote // sorted by votes descending
	TotalVotes     int
	EliminatedID   int
	EliminatedName string
	HasEliminated  bool
	IsWinnerRound  bool
}

type resultsData struct {
	Poll        models.Poll
	Rounds      []roundView
	WinnerID    int
	WinnerName  string
	BallotCount int
	HasBallots  bool
}
