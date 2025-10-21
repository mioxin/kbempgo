package backend

import (
	"fmt"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	gsrv "github.com/mioxin/kbempgo/pkg/grpc_server"
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
		Lg:                e.Globals.Log.With("new", "grpc"),
	}

	sock, server, err := gsrv.NewServer(&e.Grpc, opts)

	if err != nil {
		e.Globals.Log.Error("failed creating server", "error", err)
		return err
	}

	kbv1.RegisterStorServer(server, &PStor{})
	e.Globals.Log.Info("Starting gRPC listener on " + e.Grpc.Listen)

	go func() {
		defer func() {
			e.Globals.Log.Info("Stopping service...")
			server.GracefulStop()
			sock.Close()
		}()

		err := server.Serve(sock)
		if err != nil {
			e.Globals.Log.Error("Failed to serve", "error", err)
		}
	}()
	if e.Grpc.Listen == "" {
		return fmt.Errorf("listen addr of gRPC gateway not congigured")
	}

	// start gRPC Gateway server
	gwOpts := &gsrv.GatewayOptions{
		WithPrometheus: true,
	}

	gw, err := gsrv.NewGateway(&e.GrpcProxy, gwOpts)
	if err != nil {
		e.Globals.Log.Error("gRPC Proxy failed to construct", "error", err)
		return err
	}

	err = gw.Connect(e.Globals.Ctx, sock.Addr(), &e.Grpc)
	if err != nil {
		e.Globals.Log.Error("gRPC Proxy connect failed", "error", err, "remote", sock.Addr())
		return err
	}

	err = gw.RegisterAll(
		kbv1.RegisterStorHandler,
	)
	if err != nil {
		e.Globals.Log.Error("gRPC Proxy register failed", "error", err)
		return err
	}

	go func() {
		defer func() {
			e.Globals.Log.Info("Stopping gRPC proxy service...")
			gw.Stop()
		}()

		err := gw.Serve()
		if err != nil {
			e.Globals.Log.Error("Failed to serve proxy", "error", err)
		}
	}()

	// XXX TODO: control readiness
	gw.IsReady.Store(true)

	return nil
}
