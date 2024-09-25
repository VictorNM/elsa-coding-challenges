package leaderboard_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/victornm/equiz/internal/domain"
	"github.com/victornm/equiz/internal/event"
	"github.com/victornm/equiz/internal/leaderboard"
)

func TestService_UpdateLeaderboard(t *testing.T) {
	s := makeService(t)

	err := s.UpdateLeaderboard(context.Background(), domain.EventScoreUpdated{
		Score: domain.Score{
			SessionID:  "s1",
			Username:   "u1",
			TotalScore: decimal.NewFromFloat(1.1),
			UpdateTime: time.Now(),
		},
	})
	require.NoError(t, err)

	resp, err := s.GetLeaderboard(context.Background(), leaderboard.GetLeaderboardRequest{
		SessionID: "s1",
	})
	require.NoError(t, err)

	want := &domain.Leaderboard{
		SessionID: "s1",
		Entries: []domain.LeaderboardEntry{
			{Username: "u1", Score: 1.1},
		},
	}
	require.Equal(t, want, resp)
}

func TestServer_PublishLeaderboardUpdated(t *testing.T) {
	type (
		inputs struct {
			receivedEvents []domain.EventScoreUpdated
		}

		outputs struct {
			publishedEvents []domain.EventLeaderboardUpdated
		}
	)

	tests := map[string]struct {
		arrange func() inputs
		assert  func(t *testing.T, out outputs)
	}{
		"should publish correct event leaderboard.updated after receiving score.updated": {
			arrange: func() inputs {
				return inputs{
					receivedEvents: []domain.EventScoreUpdated{
						{
							Score: domain.Score{
								SessionID:  "s1",
								Username:   "u1",
								TotalScore: decimal.NewFromFloat(1.1),
								UpdateTime: time.Now(),
							},
						},
					},
				}
			},

			assert: func(t *testing.T, out outputs) {
				require.Len(t, out.publishedEvents, 1, "should receive 1 leaderboard updated event")
				require.Equal(t, domain.Leaderboard{
					SessionID: "s1",
					Entries: []domain.LeaderboardEntry{
						{Username: "u1", Score: 1.1},
					},
				}, out.publishedEvents[0].Leaderboard)
			},
		},

		"should publish 2 events leaderboard.updated after receiving events score.updated for 2 different sessions": {
			arrange: func() inputs {
				return inputs{
					receivedEvents: []domain.EventScoreUpdated{
						{
							Score: domain.Score{
								SessionID:  "s1",
								Username:   "u1",
								TotalScore: decimal.NewFromFloat(1.1),
								UpdateTime: time.Now()},
						},
						{
							Score: domain.Score{
								SessionID:  "s2",
								Username:   "u2",
								TotalScore: decimal.NewFromFloat(2.2),
								UpdateTime: time.Now(),
							},
						},
					},
				}
			},

			assert: func(t *testing.T, out outputs) {
				require.Len(t, out.publishedEvents, 2, "should receive 2 leaderboard updated event")
			},
		},

		"should publish 1 event leaderboard.updated after receiving events score.updated for the same session within the publish interval": {
			arrange: func() inputs {
				return inputs{
					receivedEvents: []domain.EventScoreUpdated{
						{
							Score: domain.Score{
								SessionID:  "s1",
								Username:   "u1",
								TotalScore: decimal.NewFromFloat(1.1),
								UpdateTime: time.Now(),
							},
						},
						{
							Score: domain.Score{
								SessionID:  "s1",
								Username:   "u2",
								TotalScore: decimal.NewFromFloat(2.2),
								UpdateTime: time.Now(),
							},
						},
					},
				}
			},

			assert: func(t *testing.T, out outputs) {
				require.Len(t, out.publishedEvents, 1, "should receive 1 leaderboard updated event")
			},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			in, out := tt.arrange(), outputs{}

			eb := event.NewBus()

			var mu sync.Mutex
			eb.Subscribe(domain.EventNameLeaderboardUpdated, func(ctx context.Context, e event.Event) error {
				mu.Lock()
				out.publishedEvents = append(out.publishedEvents, e.(domain.EventLeaderboardUpdated))
				mu.Unlock()
				return nil
			})

			s := makeService(t,
				withEventBus(eb),
			)

			for _, e := range in.receivedEvents {
				err := s.UpdateLeaderboard(context.Background(), e)
				require.NoError(t, err)
			}

			eb.Stop()

			tt.assert(t, out)
		})
	}
}

func makeService(t *testing.T, opts ...options) *leaderboard.Service {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	rs := miniredis.RunT(t)
	rc := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{rs.Addr()},
	})
	require.NoError(t, rc.Ping(ctx).Err(), "should be able to ping redis")

	c := leaderboard.Config{
		EventBus: event.NewBus(),
		Redis:    rc,
	}

	for _, opt := range opts {
		opt(&c)
	}

	return leaderboard.NewService(c)
}

type options func(c *leaderboard.Config)

func withEventBus(eb *event.Bus) options {
	return func(c *leaderboard.Config) {
		c.EventBus = eb
	}
}
