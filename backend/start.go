package backend

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	gsrv "github.com/mioxin/kbempgo/pkg/grpc_server"
)

const ProgName string = "kbsrv"

func start(e *CLI, store *PStor) error {
	ctx, cancel := context.WithTimeout(context.Background(), e.Globals.OpTimeout)
	defer cancel()

	// start gRPC server
	opts := &gsrv.ServerOptions{
		WithKeepalive:     true,
		WithPrometheus:    true,
		WithHealth:        true,
		WithValidator:     true,
		WithPingServer:    false,
		WithVersionServer: true,
		ProgramName:       ProgName,
		Lg:                e.Log.With("srv", "gRPC"),
	}

	sock, server, err := gsrv.NewServer(&e.Grpc, opts)

	if err != nil {
		e.Log.Error("failed creating server", "error", err)
		return err
	}

	defer func() {
		e.Log.Info("Stopping gRPC service...")
		server.GracefulStop()
		sock.Close()
	}()

	kbv1.RegisterStorAPIServer(server, store)
	e.Log.Info("Starting gRPC listener on " + e.Grpc.Listen)

	go func() {
		// time.Sleep(10 * time.Second)
		err := server.Serve(sock)
		if err != nil {
			e.Log.Error("Failed to serve", "error", err)
			return
		}
	}()

	// start gRPC Gateway server
	if e.GrpcProxy.Listen == "" {
		return fmt.Errorf("listen addr of gRPC gateway not congigured")
	}

	gwOpts := &gsrv.GatewayOptions{
		WithPrometheus: true,
		Lg:             e.Log.With("srv", "gRPC_proxy"),
		Ctx:            ctx,
	}

	gw, err := gsrv.NewGateway(&e.GrpcProxy, gwOpts)
	if err != nil {
		e.Log.Error("gRPC Proxy failed to construct", "error", err)
		return err
	}

	defer func() {
		e.Log.Info("Stopping gRPC proxy service...")
		gw.Stop()
	}()

	err = gw.Connect(ctx, sock.Addr(), &e.Grpc)
	if err != nil {
		e.Log.Error("gRPC Proxy connect failed", "error", err, "remote", sock.Addr())
		return err
	}

	err = gw.RegisterAll(
		kbv1.RegisterStorAPIHandler,
	)
	if err != nil {
		e.Log.Error("gRPC Proxy register failed", "error", err)
		return err
	}

	go func() {
		err := gw.Serve()
		if err != nil {
			e.Log.Error("Failed to serve proxy", "error", err)
		}
	}()

	// XXX TODO: control readiness
	gw.IsReady.Store(true)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	<-stop

	return nil
}
