package session

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/victornm/equiz/internal/domain"
	"github.com/victornm/equiz/internal/event"
)

type Config struct {
	DB       *pgxpool.Pool
	EventBus *event.Bus
}

type Service struct {
	db *pgxpool.Pool
	eb *event.Bus
}

func NewService(c Config) *Service {
	return &Service{
		db: c.DB,
		eb: c.EventBus,
	}
}

// CreateSessionRequest represents a request to create a new quiz session.
type CreateSessionRequest struct {
	// QuizMaster is the username of the quiz master.
	QuizMaster string
	// Questions is a list of questions in the quiz session.
	QuestionIDs []string
}

// CreateSession creates a new quiz session.
func (s *Service) CreateSession(ctx context.Context, req CreateSessionRequest) (*domain.Session, error) {
	ss := &domain.Session{
		QuizMaster:  req.QuizMaster,
		QuestionIDs: req.QuestionIDs,
	}

	if err := s.insertSession(ctx, ss); err != nil {
		return nil, err
	}

	return ss, nil
}

func (s *Service) insertSession(ctx context.Context, ss *domain.Session) (err error) {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate session ID: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			err = stderrors.Join(err, tx.Rollback(ctx))
		}
	}()

	const (
		insSessionStmt  = `INSERT INTO sessions (session_id, quiz_master) VALUES ($1, $2);`
		insQuestionStmt = `INSERT INTO sessions_questions (session_id, question_id) VALUES ($1, $2);`
	)

	_, err = tx.Exec(ctx, insSessionStmt, id, ss.QuizMaster)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	ss.SessionID = id.String()
	for _, q := range ss.QuestionIDs { // TODO: Batch insert
		_, err = tx.Exec(ctx, insQuestionStmt, id, q)
		if err != nil {
			return fmt.Errorf("insert question: %w", err)
		}
	}

	return tx.Commit(ctx)
}

type EndSessionRequest struct {
	SessionID string
}

func (s *Service) EndSession(ctx context.Context, req EndSessionRequest) (*domain.Session, error) {
	// Assuming this return is valid
	ss := domain.Session{
		SessionID: req.SessionID,
	}

	s.eb.Publish(ctx, domain.EventSessionEnded{
		Session: ss,
	})

	return &ss, nil
}

type ValidateSubmissionRequest struct {
	SessionID  string
	Username   string
	QuestionID string
	SubmitTime time.Time
}

type ValidateSubmissionResponse struct{}

func (*Service) ValidateSubmission(_ context.Context, _ ValidateSubmissionRequest) (*ValidateSubmissionResponse, error) {
	return &ValidateSubmissionResponse{}, nil
}
