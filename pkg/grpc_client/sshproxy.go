package grpc_client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/mioxin/kbempgo/pkg/logger"
	"go.opentelemetry.io/otel/trace"
)

// ConnProxy net.Conn wrapper that handles closing of SSH session
type ConnProxy struct {
	net.Conn

	ctx context.Context
	cf  context.CancelFunc
	cmd *exec.Cmd
}

func (s *ConnProxy) LocalAddr() net.Addr {
	return s.Conn.LocalAddr()
}

func (s *ConnProxy) RemoteAddr() net.Addr {
	return s.Conn.RemoteAddr()
}

func (s *ConnProxy) SetDeadline(t time.Time) error {
	return s.Conn.SetDeadline(t)
}

func (s *ConnProxy) SetReadDeadline(t time.Time) error {
	return s.Conn.SetReadDeadline(t)
}

func (s *ConnProxy) SetWriteDeadline(t time.Time) error {
	return s.Conn.SetWriteDeadline(t)
}

func (s *ConnProxy) Read(b []byte) (int, error) {
	return s.Conn.Read(b)
}

func (s *ConnProxy) Write(b []byte) (int, error) {
	return s.Conn.Write(b)
}

func (s *ConnProxy) Close() error {
	defer s.cf()
	return s.Conn.Close()
}

// resolve free local port
func resolveFreePort() (string, error) {
	la, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return "", fmt.Errorf("Failed to resolve: %w", err)
	}

	l, err := net.ListenTCP("tcp", la)
	if err != nil {
		return "", fmt.Errorf("Failed to listen: %w", err)
	}
	defer l.Close()
	return l.Addr().String(), nil
}

func NewConnProxyDialer(config *ClientConfig) func(context.Context, string) (net.Conn, error) {
	return func(cctx context.Context, address string) (net.Conn, error) {
		cctx, span := tracer.Start(cctx, "grpcclient/ProxyDialer", trace.WithSpanKind(trace.SpanKindClient))
		defer span.End()

		lg := logger.FromContext(cctx)

		// resolve free local port
		laddr, err := resolveFreePort()
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("Failed to resolve: %w", err)
		}
		lg.Debug("Resolved local", "laddr", laddr)

		// start ssh port forwarder
		args := []string{
			// "ssh",
			"-nNT",
			"-L",
			fmt.Sprintf("%s:%s", laddr, address),
			"-p",
			fmt.Sprintf("%d", config.SSHProxy.Port),
		}

		if config.SSHProxy.User != "" {
			args = append(args, fmt.Sprintf("%s@%s", config.SSHProxy.User, config.SSHProxy.Host))
		} else {
			args = append(args, config.SSHProxy.Host)
		}

		for _, opts := range []string{
			"ForwardAgent=yes",
			"ControlMaster=auto",
			"ControlPersist=60s",
			"UserKnownHostsFile=/dev/null",
			"StrictHostKeyChecking=no",
			"ConnectTimeout=6",
			"ConnectionAttempts=30",
			"PreferredAuthentications=publickey",
		} {
			args = append(args, "-o", opts)
		}

		if config.SSHProxy.Verbose {
			args = append(args, "-v")
		}

		ctx, cf := context.WithCancel(context.Background())
		proxy := &ConnProxy{
			ctx: ctx,
			cf:  cf,
		}

		proxy.cmd = exec.CommandContext(proxy.ctx, "ssh", args...)
		proxy.cmd.SysProcAttr = &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGTERM,
		}

		cmdLg := lg.With("cmd", "ssh "+strings.Join(args, " "))

		stdout, err := proxy.cmd.StdoutPipe()
		if err != nil {
			cf()
			span.RecordError(err)
			cmdLg.Error("Failed to make ssh tunnel stdout pipe", "error", err)
			return nil, err
		}

		stderr, err := proxy.cmd.StderrPipe()
		if err != nil {
			cf()
			stdout.Close() // nolint
			span.RecordError(err)
			cmdLg.Error("Failed to make ssh tunnel stderr pipe", "error", err)
			return nil, err
		}

		go func() {
			<-ctx.Done()

			err := errors.Join(stdout.Close(), stderr.Close())
			if err != nil {
				cmdLg.Error("Tunnel pipe close", "error", err)
			}
		}()

		go func() {
			// NOTE(vermakov): we do not expect to see any messages here
			scaner := bufio.NewScanner(stdout)
			for scaner.Scan() {
				msg := scaner.Text()
				lg.Warn("ssh:", "msg", msg)
			}
		}()

		go func() {
			scaner := bufio.NewScanner(stderr)
			for scaner.Scan() {
				msg := scaner.Text()

				if strings.HasPrefix(msg, "debug") {
					lg.Debug("ssh:", "msg", msg)
				} else {
					lg.Info("ssh:", "msg", msg)
				}
			}
		}()

		if config.SSHProxy.Verbose {
			cmdLg.Info("Starting ssh tunnel")
		} else {
			cmdLg.Debug("Starting ssh tunnel")
		}

		err = proxy.cmd.Start()
		if err != nil {
			cf()
			span.RecordError(err)
			cmdLg.Error("Failed to start ssh tunnel", "error", err)
			return nil, fmt.Errorf("Command error: %w", err)
		}

		// dial
		bck := backoff.NewExponentialBackOff()
		bck.InitialInterval = 100 * time.Millisecond
		bck.MaxInterval = 10 * time.Second
		bck.Multiplier = 2

		err = backoff.Retry(func() error {
			var d net.Dialer

			proxy.Conn, err = d.DialContext(cctx, "tcp", laddr)
			if errors.Is(err, syscall.ECONNREFUSED) {
				span.AddEvent("Dial failed, retrying...")
				lg.Debug("Dial failed, retrying after short timeout", "error", err)
				return err
			} else if err != nil {
				lg.Error("Unrecoverable dial error", "error", err)
				return &backoff.PermanentError{Err: fmt.Errorf("Dial failed: %w", err)}
			}

			return nil
		}, backoff.WithContext(bck, cctx))

		if err != nil {
			cf()
			span.RecordError(err)
			return nil, err
		}

		return proxy, nil
	}
}
