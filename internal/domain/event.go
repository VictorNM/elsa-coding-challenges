package domain

const (
	EventNameSessionEnded       = "session.ended"
	EventNameScoreUpdated       = "score.updated"
	EventNameLeaderboardUpdated = "leaderboard.updated"
)

type EventSessionEnded struct {
	Session Session
}

func (EventSessionEnded) Name() string { return EventNameSessionEnded }

type EventScoreUpdated struct {
	Score Score
}

func (EventScoreUpdated) Name() string { return EventNameScoreUpdated }

type EventLeaderboardUpdated struct {
	Leaderboard Leaderboard
}

func (EventLeaderboardUpdated) Name() string { return EventNameLeaderboardUpdated }
