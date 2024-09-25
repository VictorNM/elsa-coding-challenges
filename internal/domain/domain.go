package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Session represents a quiz session.
type Session struct {
	SessionID    string
	QuizMaster   string
	QuestionIDs  []string
	Participants []string
}

type Question struct {
	QuestionID   string
	QuestionText string
	Options      []Option
}

type Option struct {
	OptionID   string
	OptionText string
}

// Score represents a user's score within a quiz session.
type Score struct {
	SessionID  string
	Username   string
	TotalScore decimal.Decimal
	UpdateTime time.Time
}

// Leaderboard represents a list of users and their scores within a quiz session.
// The list is sorted by score in descending order.
type Leaderboard struct {
	SessionID string
	Entries   []LeaderboardEntry
}

type LeaderboardEntry struct {
	Username string
	Score    float64
}
