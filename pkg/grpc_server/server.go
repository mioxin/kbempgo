// Package grpcserver provides common server helpers
package grpc_server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"

	grpc_prom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpc_selector "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/validator"
	"github.com/mioxin/kbempgo/pkg/grpc_slog"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	zsrv "google.golang.org/grpc/channelz/service"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"gopkg.in/mcuadros/go-defaults.v1"
)

type MatchFunc func(ctx context.Context, callMeta interceptors.CallMeta) bool

// ServerOptions options enabled for server
type ServerOptions struct {
	WithKeepalive     bool `default:"true"`
	WithPrometheus    bool `default:"true"`
	WithChannelz      bool `default:"false"`
	WithReflection    bool `default:"false"`
	WithHealth        bool `default:"true"`
	WithValidator     bool `default:"false"`
	WithPingServer    bool `default:"false"`
	WithVersionServer bool `default:"false"`
	AuthFunc          grpc_auth.AuthFunc
	AuthMatchFunc     MatchFunc
	ValidateMatchFunc MatchFunc
	ProgramName       string
	ProgramVersion    string
	Lg                *slog.Logger
}

var srvMetricsInitOnce sync.Once
var srvMetrics *grpc_prom.ServerMetrics

// NewServer creates new gRPC server
func NewServer(config *ServerConfig, opts *ServerOptions) (net.Listener, *grpc.Server, error) {
	if opts == nil {
		opts = &ServerOptions{}
		defaults.SetDefaults(opts)
	}
	if opts.ProgramName == "" {
		name, err := os.Executable()
		if err != nil {
			return nil, nil, fmt.Errorf("get executable name error: %w", err)
		}
		opts.ProgramName = name
	}
	if opts.Lg == nil {
		opts.Lg = slog.Default()
	}

	glgOpts := []grpc_logging.Option{
		grpc_logging.WithLogOnEvents(grpc_logging.FinishCall),
	}
	glg := grpc_slog.InterceptorLogger(opts.Lg)

	srvOpts := []grpc.ServerOption{}
	streamInt := []grpc.StreamServerInterceptor{
		grpc_logging.StreamServerInterceptor(glg, glgOpts...),
	}
	unaryInt := []grpc.UnaryServerInterceptor{
		grpc_logging.UnaryServerInterceptor(glg, glgOpts...),
	}

	if config.TLS != nil {
		srvOpts = append(srvOpts, grpc.Creds(credentials.NewTLS(config.TLS)))
	}

	if opts.WithKeepalive {
		kaep := keepalive.EnforcementPolicy{
			MinTime:             config.KeepAliveEnforcementMinTime,
			PermitWithoutStream: true,
		}

		kasp := keepalive.ServerParameters{
			Time:    config.KeepAliveTime,
			Timeout: config.KeepAliveTimeout,
		}

		srvOpts = append(srvOpts, grpc.KeepaliveEnforcementPolicy(kaep))
		srvOpts = append(srvOpts, grpc.KeepaliveParams(kasp))
	}

	if opts.WithPrometheus {
		srvMetricsInitOnce.Do(func() {
			opts.Lg.Debug("Initializing prometheus instrumentation")
			srvMetrics = grpc_prom.NewServerMetrics()
			err := prometheus.Register(srvMetrics)
			if err != nil {
				opts.Lg.Error("Failed to setup prometheus instrumentation", "error", err)
			}
		})

		if srvMetrics != nil {
			streamInt = append(streamInt, srvMetrics.StreamServerInterceptor())
			unaryInt = append(unaryInt, srvMetrics.UnaryServerInterceptor())
		} else {
			opts.Lg.Warn("Prometheus instrumentation init failed. Disabling option.")
			opts.WithPrometheus = false
		}
	}

	if opts.WithValidator {
		if opts.ValidateMatchFunc != nil {
			streamInt = append(streamInt, grpc_selector.StreamServerInterceptor(grpc_validator.StreamServerInterceptor(), grpc_selector.MatchFunc(opts.ValidateMatchFunc)))
			unaryInt = append(unaryInt, grpc_selector.UnaryServerInterceptor(grpc_validator.UnaryServerInterceptor(), grpc_selector.MatchFunc(opts.ValidateMatchFunc)))
		} else {
			streamInt = append(streamInt, grpc_validator.StreamServerInterceptor())
			unaryInt = append(unaryInt, grpc_validator.UnaryServerInterceptor())
		}
	}

	if opts.AuthFunc != nil {
		if opts.AuthMatchFunc != nil {
			streamInt = append(streamInt, grpc_selector.StreamServerInterceptor(grpc_auth.StreamServerInterceptor(opts.AuthFunc), grpc_selector.MatchFunc(opts.AuthMatchFunc)))
			unaryInt = append(unaryInt, grpc_selector.UnaryServerInterceptor(grpc_auth.UnaryServerInterceptor(opts.AuthFunc), grpc_selector.MatchFunc(opts.AuthMatchFunc)))
		} else {
			streamInt = append(streamInt, grpc_auth.StreamServerInterceptor(opts.AuthFunc))
			unaryInt = append(unaryInt, grpc_auth.UnaryServerInterceptor(opts.AuthFunc))
		}
	}

	srvOpts = append(srvOpts, grpc.ChainStreamInterceptor(streamInt...))
	srvOpts = append(srvOpts, grpc.ChainUnaryInterceptor(unaryInt...))
	srvOpts = append(srvOpts, grpc.StatsHandler(otelgrpc.NewServerHandler()))

	lg3 := opts.Lg.With("listen", config.Listen)
	lg3.Debug("server opts", "len", len(srvOpts))

	sock, err := net.Listen("tcp", config.Listen)
	if err != nil {
		lg3.Error("Could not listen", "error", err)
		return nil, nil, fmt.Errorf("listen error: %w", err)
	}
	defer func() {
		if err != nil {
			lg3.Error("Close socket because of error", "error", err)
			sock.Close()
		}
	}()

	lg3.Info("Starting gRPC")
	// nosemgrep: go.grpc.security.grpc-server-insecure-connection.grpc-server-insecure-connection
	server := grpc.NewServer(srvOpts...)

	if opts.WithReflection {
		reflection.Register(server)
	}
	if opts.WithChannelz || config.Debug {
		zsrv.RegisterChannelzServiceToServer(server)
	}
	if opts.WithHealth {
		grpc_health_v1.RegisterHealthServer(server, health.NewServer())
	}
	if opts.WithPrometheus {
		srvMetrics.InitializeMetrics(server)
	}

	return sock, server, nil
}
