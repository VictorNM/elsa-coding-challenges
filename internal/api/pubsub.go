package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"golang.org/x/sync/errgroup"

	"github.com/victornm/equiz/internal/domain"
)

const maxConcurrent = 100

type (
	Notification struct {
		Event string `json:"event"`
		Data  any    `json:"data"`
	}

	Leaderboard struct {
		SessionID string             `json:"session_id"`
		Entries   []LeaderboardEntry `json:"entries"`
	}

	LeaderboardEntry struct {
		Username string `json:"username"`
		Score    string `json:"score"`
	}
)

func (a *API) PublishLeaderboardUpdated(ctx context.Context, e domain.EventLeaderboardUpdated) error {
	l := e.Leaderboard

	data := Leaderboard{
		SessionID: l.SessionID,
		Entries:   make([]LeaderboardEntry, 0, len(l.Entries)),
	}

	for _, entry := range l.Entries {
		data.Entries = append(data.Entries, LeaderboardEntry{
			Username: entry.Username,
			Score:    strconv.FormatFloat(entry.Score, 'f', -1, 64),
		})
	}

	var eg errgroup.Group
	eg.SetLimit(maxConcurrent)

	for _, entry := range data.Entries {
		eg.Go(func() error {
			return a.publishNotification(ctx, entry.Username, e.Name(), data)
		})
	}

	return eg.Wait()
}

func (a *API) publishNotification(ctx context.Context, user, event string, data any) error {
	n := Notification{
		Event: event,
		Data:  data,
	}

	b, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("pubsub: marshal %s: %v", event, err)
	}

	return a.redis.Publish(ctx, fmt.Sprintf("%s:user:%s", a.prefix, user), b).Err()
}
