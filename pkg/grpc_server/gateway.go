package grpc_server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/mioxin/kbempgo/pkg/grpc_client"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sebest/xff"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/mcuadros/go-defaults.v1"
)

// Gateway gRPC Gateway proxy
type Gateway struct {
	Ctx     context.Context
	Conn    *grpc.ClientConn
	Server  http.Server
	IsReady atomic.Bool

	gwmux *runtime.ServeMux
	mux   *http.ServeMux
	lg    *slog.Logger
}

// GatewayOptions options for NewGateway
type GatewayOptions struct {
	WithPrometheus bool `default:"true"`
	Ctx            context.Context
	Lg             *slog.Logger
}

// NewGateway starts gRPC Gateway proxy
func NewGateway(config *ProxyConfig, opts *GatewayOptions) (*Gateway, error) {
	if opts == nil {
		opts = &GatewayOptions{}
		defaults.SetDefaults(opts)
	}
	if opts.Lg == nil {
		opts.Lg = slog.Default()
	}
	if opts.Ctx == nil {
		opts.Ctx = context.Background()
	}

	ret := &Gateway{
		Ctx: opts.Ctx,
		lg:  opts.Lg,
	}

	ret.lg.Info("Creating gRPC Proxy...", "listen", config.Listen)

	ret.gwmux = runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.HTTPBodyMarshaler{
			Marshaler: &runtime.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					UseProtoNames:   true,
					EmitUnpopulated: false,
				},
				UnmarshalOptions: protojson.UnmarshalOptions{
					DiscardUnknown: true,
				},
			},
		}),
		runtime.WithMarshalerOption("application/octet-stream", &runtime.ProtoMarshaller{}),
		runtime.WithMarshalerOption("application/x-protobuf", &runtime.ProtoMarshaller{}),
	)

	ret.mux = http.NewServeMux()
	ret.mux.HandleFunc("/-/ready", func(w http.ResponseWriter, _ *http.Request) {
		// TODO: show connection status as 502 Bad Gateway on error
		if ret.IsReady.Load() {
			// 200 Ok
			fmt.Fprintf(w, "ok\n")
		} else {
			// 503 Service Unavailable
			http.Error(w, "not ready\n", http.StatusServiceUnavailable)
		}
	})
	if opts.WithPrometheus {
		ret.mux.Handle("/metrics", promhttp.Handler())
	}
	ret.mux.Handle("/", ret.gwmux)

	var handler http.Handler = ret.mux
	if config.CORS.Enabled {
		cors := config.CORS.New()
		handler = cors.Handler(handler)
	}
	if config.UseXFF {
		xffmw, _ := xff.Default()
		handler = xffmw.Handler(handler)
	}

	ret.Server = http.Server{
		Addr:      config.Listen,
		Handler:   handler,
		TLSConfig: config.TLS,
	}

	return ret, nil
}

// Connect to the gRPC server with retry
func (gw *Gateway) Connect(ctx context.Context, addr net.Addr, srvCfg *ServerConfig) error {
	if srvCfg == nil {
		srvCfg = &ServerConfig{}
		srvCfg.SetDefaults()
	}

	connURL := addr.String()
	cliCfg := srvCfg.ClientConfig()
	cliCfg.Address = connURL

	gw.lg.Debug("Connecting to gRPC...", "url", connURL)

	//nolint:staticcheck
	conn, err := grpc_client.NewConnection(ctx, cliCfg, []grpc.DialOption{}...)
	// conn, err := gw.connectWithGRPCCeck(ctx, cliCfg, grpc.WithBlock())

	if err != nil {
		gw.lg.Error("Failed to dial gRPC server", "url", connURL, "error", err)
		return fmt.Errorf("dial error: %w", err)
	}

	gw.Conn = conn
	return nil
}

// GWRegisterer represents RegisterFooHandler funcs
type GWRegisterer func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error

// Register registers gateway handler
func (gw *Gateway) Register(f GWRegisterer) error {
	err := f(gw.Ctx, gw.gwmux, gw.Conn)
	if err != nil {
		gw.lg.Error("Failed to register gateway", "error", err)
		return fmt.Errorf("Register error: %w", err)
	}

	return nil
}

// RegisterAll registers multiple handlers
func (gw *Gateway) RegisterAll(fns ...GWRegisterer) error {
	var err error

	for _, f := range fns {
		err = errors.Join(err, gw.Register(f))
	}

	return err
}

// Serve requests
func (gw *Gateway) Serve() error {
	var err error

	if gw.Server.TLSConfig != nil {
		gw.lg.Info("Listening proxy TLS", "listen", gw.Server.Addr)
		err = gw.Server.ListenAndServeTLS("", "")
	} else {
		gw.lg.Info("Listening proxy insecure", "listen", gw.Server.Addr)
		err = gw.Server.ListenAndServe()
	}

	if errors.Is(err, http.ErrServerClosed) {
		gw.lg.Error("Proxy stopped abnormally", "error", err)
		return fmt.Errorf("Serve error: %w", err)
	}
	gw.lg.Info("Proxy stopped")
	return nil
}

// Stop stops the server
func (gw *Gateway) Stop() error {
	return gw.Server.Close()
}

// Mux provides access to serve mux
func (gw *Gateway) Mux() *http.ServeMux {
	return gw.mux
}
