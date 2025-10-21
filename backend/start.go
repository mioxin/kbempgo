package backend

import (
	"context"
	"fmt"
	"log"
	"time"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/pkg/grpc_client"
	gsrv "github.com/mioxin/kbempgo/pkg/grpc_server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const ProgName string = "kbsrv"

func startGrpc(e *CLI) error {

	// start gRPC server
	opts := &gsrv.ServerOptions{
		WithKeepalive:     true,
		WithPrometheus:    true,
		WithHealth:        true,
		WithValidator:     true,
		WithPingServer:    true,
		WithVersionServer: true,
		ProgramName:       ProgName,
		Lg:                e.Log.With("new", "grpc"),
	}

	sock, server, err := gsrv.NewServer(&e.Grpc, opts)

	if err != nil {
		e.Log.Error("failed creating server", "error", err)
		return err
	}

	kbv1.RegisterStorServer(server, &PStor{})
	e.Log.Info("Starting gRPC listener on " + e.Grpc.Listen)

	go func() {
		defer func() {
			e.Log.Info("Stopping service...")
			server.GracefulStop()
			sock.Close()
		}()

		err := server.Serve(sock)
		if err != nil {
			e.Log.Error("Failed to serve", "error", err)
			return
		}
	}()

	// wait gRPC server starting
	_, err = waitForGRPCHealth(e.Ctx, e.Grpc.ClientConfig())

	if err != nil {
		e.Log.Error("wait gRPC server start", "error", err)
		return err
	}

	// start gRPC Gateway server
	if e.GrpcProxy.Listen == "" {
		return fmt.Errorf("listen addr of gRPC gateway not congigured")
	}

	gwOpts := &gsrv.GatewayOptions{
		WithPrometheus: true,
		Lg:             e.Log.With("new", "grpc gateway"),
		Ctx:            e.Ctx,
	}

	gw, err := gsrv.NewGateway(&e.GrpcProxy, gwOpts)
	if err != nil {
		e.Log.Error("gRPC Proxy failed to construct", "error", err)
		return err
	}

	err = gw.Connect(e.Ctx, sock.Addr(), &e.Grpc)
	if err != nil {
		e.Log.Error("gRPC Proxy connect failed", "error", err, "remote", sock.Addr())
		return err
	}

	err = gw.RegisterAll(
		kbv1.RegisterStorHandler,
	)
	if err != nil {
		e.Log.Error("gRPC Proxy register failed", "error", err)
		return err
	}

	go func() {
		defer func() {
			e.Log.Info("Stopping gRPC proxy service...")
			gw.Stop()
		}()

		err := gw.Serve()
		if err != nil {
			e.Log.Error("Failed to serve proxy", "error", err)
		}
	}()

	// XXX TODO: control readiness
	gw.IsReady.Store(true)

	return nil
}

// waitForGRPCHealth: polling Health/Check
func waitForGRPCHealth(cx context.Context, cliConfig *grpc_client.ClientConfig) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(cx, 5*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			log.Printf("gRPC Health check canceled ...")
			return nil, ctx.Err()

		default:
			conn, err := grpc_client.NewConnection(ctx, cliConfig, grpc.WithTransportCredentials(insecure.NewCredentials()))
			addr := cliConfig.Address

			if err != nil {
				log.Printf("Waiting for gRPC connection (%s): %v. Retry in 1s...", addr, err)
				time.Sleep(1 * time.Second)
				continue
			}

			// check health through RPC
			healthClient := grpc_health_v1.NewHealthClient(conn)
			healthCtx, healthCancel := context.WithTimeout(ctx, 2*time.Second)

			resp, err := healthClient.Check(healthCtx, &grpc_health_v1.HealthCheckRequest{Service: ""})
			healthCancel()
			if err != nil {
				log.Printf("Health check failed (%s): %v. Retry in 1s...", addr, err)
				conn.Close()
				time.Sleep(1 * time.Second)
				continue
			}
			if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
				log.Printf("Health status not SERVING (%s): %v. Retry in 1s...", addr, resp.Status)
				conn.Close()
				time.Sleep(1 * time.Second)
				continue
			}

			log.Printf("gRPC server healthy on %s", addr)
			return conn, nil // ready
		}
	}
}
