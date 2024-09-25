package score

import (
	"context"
	"crypto/rand"
	stderrors "errors"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/victornm/equiz/internal/domain"
	"github.com/victornm/equiz/internal/errors"
	"github.com/victornm/equiz/internal/event"
)

type Config struct {
	EventBus *event.Bus
	DB       *pgxpool.Pool
}

type Service struct {
	eb *event.Bus
	db *pgxpool.Pool
}

func NewService(c Config) *Service {
	return &Service{
		eb: c.EventBus,
		db: c.DB,
	}
}

type SubmitAnswerRequest struct {
	SessionID  string
	Username   string
	QuestionID string
	Answer     string
	SubmitTime time.Time
}

type SubmitAnswerResponse struct {
	Score      decimal.Decimal
	TotalScore decimal.Decimal
}

// SubmitAnswer increases the score of a user in a session, and return the total score of the user if successful.
func (s *Service) SubmitAnswer(ctx context.Context, req SubmitAnswerRequest) (*SubmitAnswerResponse, error) {
	// Calculate the score based on the answer. Simulate a random score between 0 and 1.
	r, _ := rand.Int(rand.Reader, new(big.Int).SetInt64(2))
	score := decimal.NewFromBigInt(r, 0)
	total, err := s.insertScore(ctx, req, score)
	if err != nil {
		return nil, err
	}

	dto := domain.Score{
		SessionID:  req.SessionID,
		Username:   req.Username,
		TotalScore: total,
		UpdateTime: req.SubmitTime,
	}

	s.eb.Publish(ctx, domain.EventScoreUpdated{
		Score: dto,
	})

	return &SubmitAnswerResponse{
		Score:      score,
		TotalScore: total,
	}, nil
}

func (s *Service) insertScore(ctx context.Context, req SubmitAnswerRequest, score decimal.Decimal) (decimal.Decimal, error) {
	const stmt = `
WITH inserted AS (
	INSERT INTO scores (session_id, username, question_id, score, create_time)
	VALUES ($1, $2, $3, $4, $5)
)
SELECT COALESCE(SUM(score), 0) AS score FROM scores WHERE session_id = $1 AND username = $2;`

	var total decimal.Decimal
	err := s.db.QueryRow(ctx, stmt, req.SessionID, req.Username, req.QuestionID, score, req.SubmitTime).Scan(&total)

	var pgErr *pgconn.PgError
	const codeUniqueViolation = "23505"
	if stderrors.As(err, &pgErr) && pgErr.Code == codeUniqueViolation {
		return decimal.Zero, errors.New(errors.CodeAlreadyExists,
			errors.WithCause(err))
	}

	if err != nil {
		return decimal.Zero, err
	}

	return total.Add(score), nil
}

type ListScoresRequest struct {
	SessionID string
}

func (s *Service) ListScores(ctx context.Context, req ListScoresRequest) ([]domain.Score, error) {
	const stmt = `
SELECT username, SUM(score) AS score 
FROM scores 
WHERE session_id = $1 
GROUP BY username
ORDER BY score DESC;`

	rows, err := s.db.Query(ctx, stmt, req.SessionID)
	if err != nil {
		return nil, err
	}

	scores, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Score, error) {
		var sc domain.Score
		if err := r.Scan(&sc.Username, &sc.TotalScore); err != nil {
			return domain.Score{}, err
		}
		sc.SessionID = req.SessionID
		return sc, nil
	})
	if err != nil {
		return nil, err
	}

	return scores, nil
}
