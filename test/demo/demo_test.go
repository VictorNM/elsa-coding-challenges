//go:build integration_test

package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/victornm/equiz/internal/api"
	equizv1 "github.com/victornm/equiz/internal/api/proto/equiz/v1"
	"github.com/victornm/equiz/internal/domain"
)

const (
	addr = "localhost:8081"
)

func TestQuiz(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		qc = makeQuizClient(t)
		wg = new(sync.WaitGroup)
	)

	var (
		session    string
		quizMaster = "quizmaster"
		questions  = []string{"q1", "q2", "q3"}
		users      = []string{"u1", "u2", "u3"}
	)

	// Prepare Redis subscriber
	subscribeAsUser(t, makeRedis(t), wg, "u1")

	// Create new session
	{
		resp, err := qc.CreateSession(ctx, &equizv1.CreateSessionRequest{
			RequestId:   uuid.New().String(),
			QuizMaster:  quizMaster,
			QuestionIds: questions,
		})
		require.NoError(t, err)
		session = resp.Session.SessionId
	}

	// For each question, all users will submit answers concurrently
	for _, q := range questions {
		t.Logf("Starting question %q", q)
		var eg errgroup.Group
		for _, u := range users {
			u := u
			eg.Go(func() error {
				resp, err := qc.SubmitAnswer(ctx, &equizv1.SubmitAnswerRequest{
					RequestId:  uuid.New().String(),
					SessionId:  session,
					Username:   u,
					QuestionId: q,
					Answer:     "A",
					SubmitTime: timestamppb.Now(),
				})
				if err != nil {
					return fmt.Errorf("user %q submit answer: %w", u, err)
				}

				t.Logf("User %q submitted answer: score=%.2f, total_score=%.2f", u, resp.Score, resp.TotalScore)
				return nil
			})
		}

		err := eg.Wait()
		require.NoError(t, err)

		time.Sleep(2 * time.Second)
	}

	wg.Wait()
}

func makeQuizClient(t *testing.T) equizv1.QuizServiceClient {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	return equizv1.NewQuizServiceClient(conn)
}

func subscribeAsUser(t *testing.T, rc redis.UniversalClient, wg *sync.WaitGroup, u string) {
	wg.Add(1)
	sub := subscribeRedis(t, rc, fmt.Sprintf("local:pubsub:user:%s", u))
	go func() {
		defer wg.Done()

		for msg := range sub {
			var n struct {
				Event string          `json:"event"`
				Data  json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal([]byte(msg.Payload), &n); err != nil {
				t.Logf("unmarshal notification: %v", err)
				continue
			}

			switch n.Event {
			case domain.EventNameLeaderboardUpdated:
				var l api.Leaderboard
				if err := json.Unmarshal(n.Data, &l); err != nil {
					t.Logf("unmarshal leaderboard: %v", err)
					continue
				}

				t.Logf("%s leaderboard:\n%s", u, formatLeaderboard(l))
			}
		}
	}()
}

func subscribeRedis(t *testing.T, rc redis.UniversalClient, pattern string) <-chan *redis.Message {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sub := rc.PSubscribe(ctx, pattern)
	t.Cleanup(func() { sub.Close() })

	c := make(chan *redis.Message)
	go func() {
		defer close(c)

		for {
			msg, err := sub.ReceiveMessage(ctx)
			if err != nil {
				t.Log(err)
				return
			}

			c <- msg
		}
	}()

	return c
}

func makeRedis(t *testing.T) redis.UniversalClient {
	r := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{"localhost:6379"},
	})
	t.Cleanup(func() { r.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}

	return r
}

func formatLeaderboard(l api.Leaderboard) string {
	var s string
	for _, e := range l.Entries {
		s += fmt.Sprintf("%s: %s\n", e.Username, e.Score)
	}
	return s
}
