package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

func MonitorRedis(r redis.UniversalClient) error {
	if err := redisotel.InstrumentTracing(r); err != nil {
		return fmt.Errorf("instrument tracing: %w", err)
	}
	if err := redisotel.InstrumentMetrics(r); err != nil {
		return fmt.Errorf("instrument metrics: %w", err)
	}
	r.AddHook(redisLog{})
	return nil
}

type redisLog struct{}

func (redisLog) DialHook(hook redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		slog.InfoContext(ctx, fmt.Sprintf("redis: dialing %s %s", network, addr))
		conn, err := hook(ctx, network, addr)
		slog.InfoContext(ctx, fmt.Sprintf("redis: finished dialing %s %s", network, addr))
		return conn, err
	}
}

func (redisLog) ProcessHook(hook redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		slog.InfoContext(ctx, fmt.Sprintf("redis: starting processing: <%s>", cmd))
		err := hook(ctx, cmd)
		slog.InfoContext(ctx, fmt.Sprintf("redis: finished processing: <%s>", cmd.String()))
		return err
	}
}

func (redisLog) ProcessPipelineHook(hook redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		slog.InfoContext(ctx, fmt.Sprintf("redis: pipeline starting processing: %v", cmds))
		err := hook(ctx, cmds)
		slog.InfoContext(ctx, fmt.Sprintf("redis: pipeline finished processing: %v", cmds))
		return err
	}
}
