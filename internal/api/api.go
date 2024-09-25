package api

import (
	"context"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	equizv1 "github.com/victornm/equiz/internal/api/proto/equiz/v1"
	"github.com/victornm/equiz/internal/domain"
	"github.com/victornm/equiz/internal/errors"
	"github.com/victornm/equiz/internal/event"
	"github.com/victornm/equiz/internal/leaderboard"
	"github.com/victornm/equiz/internal/score"
	"github.com/victornm/equiz/internal/session"
)

type Config struct {
	GRPC         *grpc.Server
	EventBus     *event.Bus
	Session      *session.Service
	Score        *score.Service
	Leaderboard  *leaderboard.Service
	Redis        Redis
	PubsubPrefix string
}

type Redis interface {
	Publish(ctx context.Context, channel string, message any) *redis.IntCmd
}

type API struct {
	equizv1.UnimplementedQuizServiceServer

	qss *session.Service
	ss  *score.Service
	ls  *leaderboard.Service

	redis  Redis
	prefix string
}

func New(c Config) *API {
	a := &API{
		qss:    c.Session,
		ss:     c.Score,
		ls:     c.Leaderboard,
		redis:  c.Redis,
		prefix: c.PubsubPrefix,
	}

	// gRPC APIs
	equizv1.RegisterQuizServiceServer(c.GRPC, a)

	// Register event handlers
	c.EventBus.Subscribe(domain.EventNameLeaderboardUpdated, func(ctx context.Context, e event.Event) error {
		return a.PublishLeaderboardUpdated(ctx, e.(domain.EventLeaderboardUpdated))
	})

	return a
}

func (a *API) CreateSession(ctx context.Context, req *equizv1.CreateSessionRequest) (*equizv1.CreateSessionResponse, error) {
	ss, err := a.qss.CreateSession(ctx, session.CreateSessionRequest{
		QuizMaster:  req.QuizMaster,
		QuestionIDs: req.QuestionIds,
	})
	if err != nil {
		return nil, err
	}

	resp := &equizv1.CreateSessionResponse{
		Session: &equizv1.Session{
			SessionId:   ss.SessionID,
			QuizMaster:  ss.QuizMaster,
			QuestionIds: ss.QuestionIDs,
		},
	}

	return resp, nil
}

func (a *API) SubmitAnswer(ctx context.Context, req *equizv1.SubmitAnswerRequest) (*equizv1.SubmitAnswerResponse, error) {
	_, err := a.qss.ValidateSubmission(ctx, session.ValidateSubmissionRequest{
		SessionID:  req.SessionId,
		Username:   req.Username,
		QuestionID: req.QuestionId,
		SubmitTime: req.SubmitTime.AsTime(),
	})
	if err != nil {
		return nil, err
	}

	sc, err := a.ss.SubmitAnswer(ctx, score.SubmitAnswerRequest{
		SessionID:  req.SessionId,
		Username:   req.Username,
		QuestionID: req.QuestionId,
		Answer:     req.Answer,
		SubmitTime: req.SubmitTime.AsTime(),
	})
	if err != nil {
		return nil, err
	}

	if e := errors.Convert(err); e.Code == errors.CodeAlreadyExists {
		return nil, errors.New(errors.CodeAlreadyExists,
			errors.WithMessagef("answer is already submitted: session=%s username=%s, question=%s", req.SessionId, req.Username, req.QuestionId),
			errors.WithCause(e.Unwrap()),
		)
	}

	return &equizv1.SubmitAnswerResponse{
		Score:      sc.Score.InexactFloat64(),
		TotalScore: sc.TotalScore.InexactFloat64(),
	}, nil
}

func (a *API) GetLeaderboard(ctx context.Context, req *equizv1.GetLeaderboardRequest) (*equizv1.GetLeaderboardResponse, error) {
	l, err := a.ls.GetLeaderboard(ctx, leaderboard.GetLeaderboardRequest{
		SessionID: req.SessionId,
	})
	if err != nil {
		return nil, err
	}

	resp := &equizv1.GetLeaderboardResponse{
		Leaderboard: &equizv1.Leaderboard{
			SessionId: l.SessionID,
			Entries:   make([]*equizv1.LeaderboardEntry, 0, len(l.Entries)),
		},
	}

	for _, e := range l.Entries {
		resp.Leaderboard.Entries = append(resp.Leaderboard.Entries, &equizv1.LeaderboardEntry{
			Username: e.Username,
			Score:    e.Score,
		})
	}

	return resp, nil
}
