// Package grpcclient provides common client helpers
package grpc_client

import (
	"context"
	"fmt"
	"time"

	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/mioxin/kbempgo/pkg/grpc_slog"
	"github.com/mioxin/kbempgo/pkg/logger"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

var tracer = otel.Tracer("gRPC-Client")

// NewConnection creates connection to gRPC server
func NewConnection(ctx context.Context, config *ClientConfig, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	lg := logger.FromContextOrNop(ctx)
	lg2 := lg.With("proto", "tcp")

	// XXX: support for several endpoints
	target := config.GetAddress()

	authDialOption := grpc.WithTransportCredentials(insecure.NewCredentials())
	if config.TLS != nil {
		creds := credentials.NewTLS(config.TLS)
		authDialOption = grpc.WithTransportCredentials(creds)
		lg2 = lg.With("proto", "tls")
	}

	dialOpts := append([]grpc.DialOption{authDialOption}, opts...)
	if config.DialKeepAliveTime > 0 {
		dialOpts = append(dialOpts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                config.DialKeepAliveTime,
			Timeout:             config.DialKeepAliveTimeout,
			PermitWithoutStream: config.PermitWithoutStream,
		}))
	}

	lg2 = lg2.With("endpoint", target)

	glgOpts := []grpc_logging.Option{
		grpc_logging.WithLogOnEvents(grpc_logging.FinishCall),
	}
	glg := grpc_slog.InterceptorLogger(lg2)

	streamInt := []grpc.StreamClientInterceptor{
		grpc_logging.StreamClientInterceptor(glg, glgOpts...),
	}
	unaryInt := []grpc.UnaryClientInterceptor{
		grpc_logging.UnaryClientInterceptor(glg, glgOpts...),
	}

	dialOpts = append(dialOpts, grpc.WithChainStreamInterceptor(streamInt...))
	dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(unaryInt...))
	dialOpts = append(dialOpts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))

	if config.DialTimeout > 0 {
		dialOpts = append(dialOpts, grpc.WithIdleTimeout(config.DialTimeout))
	} else {
		dialOpts = append(dialOpts, grpc.WithIdleTimeout(10*time.Second))
	}

	if config.SSHProxy.Host != "" {
		dialOpts = append(dialOpts, grpc.WithContextDialer(NewConnProxyDialer(config)))
	}

	// nolint:staticcheck

	conn, err := grpc.NewClient(target, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("can't create client connection: %w", err)
	}

	err = Check(ctx, conn, "")
	if err != nil {
		return nil, fmt.Errorf("error check health: %w", err)
	}

	return conn, nil
}

func Check(ctx context.Context, conn *grpc.ClientConn, service string) error {
	lg := logger.FromContextOrNop(ctx)
	srv := service
	if service == "" {
		srv = "general"
	}
	lg2 := lg.With("srv", srv)

	healthClient := grpc_health_v1.NewHealthClient(conn)
	// resp1, err := healthClient.List(ctx, &grpc_health_v1.HealthListRequest{})
	// lg2.Info("list", "resp", resp1, "err", err)

	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: service}) // Service name in proto
	if err != nil {
		return fmt.Errorf("health check failed: %w, Target: %s, Service: %s", err, conn.Target(), srv)
	}

	status := resp.GetStatus()
	if status == grpc_health_v1.HealthCheckResponse_SERVING {
		lg2.Info("Health of gRPC service Store is OK...")
	} else {
		lg2.Error("Health of gRPC service Store", "status", status)
		return fmt.Errorf("health check status %v, Target: %s, Service: %s", status, conn.Target(), srv)
	}

	return nil
}
