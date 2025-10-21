// Package redis provides helpers to make Redis client
package redis

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mioxin/kbempgo/pkg/logger"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// Nil reply from Redis when key does not exist
const Nil = redis.Nil

// UniversalClient alias for redis.UniversalClient
type UniversalClient = redis.UniversalClient

// ClientOptions additional options for redis
type ClientOptions struct {
	Lg *slog.Logger
}

// NewUniversalClient creates universal client to simple redis / sentinel / cluster
func NewUniversalClient(config *ClientConfig, opts *ClientOptions) (UniversalClient, error) {
	if opts == nil {
		opts = &ClientOptions{}
	}
	if opts.Lg == nil {
		opts.Lg = slog.Default()
	}

	redis.SetLogger(logger.NewCtxPrinterAt(opts.Lg, slog.LevelDebug))

	rdb := redis.NewUniversalClient(config.UniversalOptions())

	rs, err := rdb.Ping(context.TODO()).Result()
	if err != nil {
		rdb.Close()
		opts.Lg.Error("Ping error", "rs", rs, "error", err)
		return nil, fmt.Errorf("Ping error: %w", err)
	}

	if config.OtelTracing {
		err = redisotel.InstrumentTracing(rdb)
		if err != nil {
			return nil, err
		}
	}

	return rdb, nil
}
