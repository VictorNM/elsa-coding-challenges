package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/victornm/equiz/internal/api"
	"github.com/victornm/equiz/internal/event"
	"github.com/victornm/equiz/internal/leaderboard"
	"github.com/victornm/equiz/internal/score"
	"github.com/victornm/equiz/internal/session"
	"github.com/victornm/equiz/internal/telemetry"
)

type Config struct {
	HTTP struct {
		Port int32
	}

	GRPC struct {
		Port int32
	}

	Redis struct {
		Leaderboard struct {
			Addrs  []string
			Pass   string
			Prefix string
		}

		Pubsub struct {
			Addrs  []string
			Pass   string
			Prefix string
		}
	}

	Postgres struct {
		Session struct {
			Addr string
			User string
			Pass string
			Name string
		}

		Score struct {
			Addr string
			User string
			Pass string
			Name string
		}
	}
}

type Server struct {
	c Config

	eb *event.Bus

	infra struct {
		redis struct {
			leaderboard redis.UniversalClient
			pubsub      redis.UniversalClient
		}

		postgres struct {
			session *pgxpool.Pool
			score   *pgxpool.Pool
		}
	}

	service struct {
		session     *session.Service
		score       *score.Service
		leaderboard *leaderboard.Service
	}

	http *http.Server
	grpc *grpc.Server
}

func Init(c Config) (*Server, error) {
	s := &Server{c: c}

	// TODO: add telemetry

	s.eb = event.NewBus()

	if err := s.initInfra(); err != nil {
		return nil, fmt.Errorf("server: init infra: %w", err)
	}

	s.initService()
	s.initAPI()
	return s, nil
}

func (s *Server) initInfra() error {
	if err := s.initRedis(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}

	if err := s.initPostgres(); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}

	return nil
}

func (s *Server) initRedis() error {
	connect := func(addrs []string, pass string) (redis.UniversalClient, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		r := redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:    addrs,
			Password: pass,
		})

		if err := telemetry.MonitorRedis(r); err != nil {
			return nil, err
		}

		if err := r.Ping(ctx).Err(); err != nil {
			return nil, err
		}

		return r, nil
	}

	var err error
	s.infra.redis.leaderboard, err = connect(s.c.Redis.Leaderboard.Addrs, s.c.Redis.Leaderboard.Pass)
	if err != nil {
		return fmt.Errorf("leaderboard: %w", err)
	}

	s.infra.redis.pubsub, err = connect(s.c.Redis.Pubsub.Addrs, s.c.Redis.Pubsub.Pass)
	if err != nil {
		return fmt.Errorf("pubsub: %w", err)
	}

	return nil
}

func (s *Server) initPostgres() (err error) {
	connect := func(addr, user, pass, name string) (*pgxpool.Pool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cc, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://%s:%s@%s/%s", user, pass, addr, name))
		if err != nil {
			return nil, err
		}

		db, err := pgxpool.NewWithConfig(ctx, cc)
		if err != nil {
			return nil, err
		}

		if err := db.Ping(ctx); err != nil {
			return nil, err
		}

		return db, nil
	}

	s.infra.postgres.session, err = connect(s.c.Postgres.Session.Addr, s.c.Postgres.Session.User, s.c.Postgres.Session.Pass, s.c.Postgres.Session.Name)
	if err != nil {
		return fmt.Errorf("postgres: session: %w", err)
	}

	s.infra.postgres.score, err = connect(s.c.Postgres.Score.Addr, s.c.Postgres.Score.User, s.c.Postgres.Score.Pass, s.c.Postgres.Score.Name)
	if err != nil {
		return fmt.Errorf("postgres: score: %w", err)
	}

	return nil
}

func (s *Server) initService() {
	s.service.score = score.NewService(score.Config{
		EventBus: s.eb,
		DB:       s.infra.postgres.score,
	})

	s.service.session = session.NewService(session.Config{
		DB: s.infra.postgres.session,
	})

	s.service.leaderboard = leaderboard.NewService(leaderboard.Config{
		EventBus: s.eb,
		Score:    s.service.score,
		Redis:    s.infra.redis.leaderboard,
		Prefix:   s.c.Redis.Leaderboard.Prefix,
	})
}

func (s *Server) initAPI() {
	e := gin.New()
	e.GET("/metrics", gin.WrapH(promhttp.Handler()))
	pprof.Register(e, "/debug/pprof")
	e.Use(gin.Recovery())

	s.grpc = grpc.NewServer(telemetry.GRPCServerInterceptor())

	api.New(api.Config{
		GRPC:         s.grpc,
		EventBus:     s.eb,
		Session:      s.service.session,
		Score:        s.service.score,
		Leaderboard:  s.service.leaderboard,
		Redis:        s.infra.redis.pubsub,
		PubsubPrefix: s.c.Redis.Pubsub.Prefix,
	})

	s.http = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.c.HTTP.Port),
		Handler:           e,
		ReadHeaderTimeout: 60 * time.Second,
	}
}

func (s *Server) Start() {
	ctx := context.TODO()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.c.GRPC.Port))
	if err != nil {
		slog.ErrorContext(ctx, "grpc server: listen failed", "error", err)
		panic(err)
	}

	var eg errgroup.Group
	eg.Go(func() error {
		slog.InfoContext(ctx, fmt.Sprintf("server: gRPC listening on port %d", s.c.GRPC.Port))
		return s.grpc.Serve(lis)
	})

	eg.Go(func() error {
		slog.InfoContext(ctx, fmt.Sprintf("server: HTTP listening on port %d", s.c.HTTP.Port))
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	err = eg.Wait()
	if err != nil {
		slog.ErrorContext(ctx, "server: shutdown with error", "error", err)
	}
}

func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.grpc.GracefulStop()
	if err := s.http.Shutdown(ctx); err != nil {
		slog.ErrorContext(ctx, "server: shutdown HTTP failed", "error", err)
	}

	s.eb.Stop()

	slog.InfoContext(ctx, "server: shutdown completed")
}
