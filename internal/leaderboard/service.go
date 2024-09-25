package leaderboard

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/victornm/equiz/internal/domain"
	"github.com/victornm/equiz/internal/errors"
	"github.com/victornm/equiz/internal/event"
	"github.com/victornm/equiz/internal/score"
)

const (
	publishInterval = 200 * time.Millisecond
)

type Config struct {
	EventBus      *event.Bus
	Score         *score.Service
	Redis         redis.UniversalClient
	Prefix        string
	NewTickerFunc func(d time.Duration) Ticker
}

type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type Service struct {
	eb     *event.Bus
	score  *score.Service
	redis  redis.UniversalClient
	prefix string
}

func NewService(c Config) *Service {
	s := &Service{
		eb:     c.EventBus,
		score:  c.Score,
		redis:  c.Redis,
		prefix: c.Prefix,
	}

	s.eb.Subscribe(domain.EventNameScoreUpdated, func(ctx context.Context, e event.Event) error {
		return s.UpdateLeaderboard(ctx, e.(domain.EventScoreUpdated))
	})

	return s
}

type GetLeaderboardRequest struct {
	SessionID string
}

// GetLeaderboard returns the leaderboard for a session, including all users and their scores.
func (s *Service) GetLeaderboard(ctx context.Context, req GetLeaderboardRequest) (*domain.Leaderboard, error) {
	res, err := s.redis.ZRevRangeWithScores(ctx, s.getLeaderboardKey(req.SessionID), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("get leaderboard: %w", err)
	}

	if len(res) == 0 {
		return nil, errors.New(errors.CodeNotFound, errors.WithMessagef("leaderboard not found: session=%s", req.SessionID))
	}

	scores := make([]domain.LeaderboardEntry, 0, len(res))
	for _, z := range res {
		scores = append(scores, domain.LeaderboardEntry{
			Username: z.Member.(string),
			Score:    z.Score,
		})
	}

	return &domain.Leaderboard{
		SessionID: req.SessionID,
		Entries:   scores,
	}, nil
}

// UpdateLeaderboard overwrites the user's score in the leaderboard.
func (s *Service) UpdateLeaderboard(ctx context.Context, e domain.EventScoreUpdated) error {
	sc := e.Score

	// TODO: retry on error
	if err := s.redis.ZAdd(ctx, s.getLeaderboardKey(sc.SessionID), redis.Z{
		Score:  sc.TotalScore.InexactFloat64(),
		Member: sc.Username,
	}).Err(); err != nil {
		return fmt.Errorf("update leaderboard: %w", err)
	}

	return s.schedulePublishLeaderboard(ctx, sc)
}

// schedulePublishLeaderboard publishes the leaderboard changes after a certain interval.
// Instead of publishing leaderboard changes immediately, publishes them after a certain interval.
// Because there are many user's scores updated in a short time, this can reduce the number of published events.
func (s *Service) schedulePublishLeaderboard(ctx context.Context, sc domain.Score) error {
	// This is a simple way to prevent multiple instances of the service from publishing the leaderboard.
	// But it's not perfect and can be improved.
	ok, err := s.redis.SetNX(ctx, s.getLeaderboardTimeKey(sc.SessionID), sc.UpdateTime.UnixMilli(), publishInterval).Result()
	if err != nil {
		return fmt.Errorf("setnx: %w", err)
	}

	if !ok {
		return nil
	}

	return s.publishLeaderboard(ctx, sc)
}

func (s *Service) publishLeaderboard(ctx context.Context, sc domain.Score) error {
	l, err := s.GetLeaderboard(ctx, GetLeaderboardRequest{
		SessionID: sc.SessionID,
	})
	if err != nil {
		return fmt.Errorf("get leaderboard failed: session=%s: %w", sc.SessionID, err)
	}

	s.eb.Publish(ctx, domain.EventLeaderboardUpdated{
		Leaderboard: *l,
	})

	return s.redis.Set(ctx, s.getLeaderboardTimeKey(sc.SessionID), sc.UpdateTime.UnixMilli(), publishInterval).Err()
}

func (s *Service) getLeaderboardKey(session string) string {
	return fmt.Sprintf("%s:%s:leaderboard", s.prefix, session)
}

func (s *Service) getLeaderboardTimeKey(session string) string {
	return fmt.Sprintf("%s:%s:time", s.prefix, session)
}
