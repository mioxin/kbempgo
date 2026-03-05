package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/mioxin/kbempgo/pkg/grpc_client"
	gsrv "github.com/mioxin/kbempgo/pkg/grpc_server"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
)

type syncCommand struct {
	Workers    int               `name:"workers" short:"w" default:"5" env:"KB_WORKERS" help:"Number of workers. Every worker run 3 goroutines."`
	Limit      int               `name:"limit" short:"l" default:"0" env:"KB_LIMIT" help:"Limit of data for get. If =0 then no limit."`
	RootRazd   string            `name:"rootr" env:"KB_ROOT_RAZD" help:"Name of root section"`
	FileSource string            `name:"file_source" default:"" help:"Path includes dep.json and sotr.json for insert data from ones into storage"`
	Grpc       gsrv.ServerConfig `embed:"" json:"grpc" prefix:"grpc-"`

	grpcClient  *grpc.ClientConn `kong:"-"`
	Lg          *slog.Logger     `kong:"-"`
	DepsCounter atomic.Int32     `kong:"-"`
	SotrCounter atomic.Int32     `kong:"-"`
}

func (e *syncCommand) Run(cli *CLI) error {
	var err error

	e.Lg = cli.Log.With("cmd", "employ")

	ctx, cancel := context.WithTimeout(context.Background(), cli.OpTimeout)
	defer cancel()
	// ****************************************
	// if FileSource setted then load data from file source to storage by gRPC
	// ****************************************
	if e.FileSource != "" {
		err := e.LoadDataToStor(ctx)
		e.Lg.Info("Loaded from", "dir", e.FileSource, "dep_count", e.DepsCounter.Load(), "sotr_count", e.SotrCounter.Load())
		return err
	}

	// ****************************************
	// Sync data from web source to storage by gRPC
	// ****************************************

	return err
}

func (e *syncCommand) LoadDataToStor(ctx context.Context) (err error) {
	cliCfg := e.Grpc.ClientConfig()
	if cliCfg.Address == "" {
		return fmt.Errorf("gRPC endpoint non configured. Config: %v", e.Grpc)
	}

	serviceConfig := `{
	"healthCheckConfig": {
		"serviceName": "kb.v1.Store1"
	}
}`

	e.grpcClient, err = grpc_client.NewConnection(ctx, cliCfg, []grpc.DialOption{grpc.WithDefaultServiceConfig(serviceConfig)}...)
	if err != nil {
		return err
	}

	defer e.grpcClient.Close()

	// Check gRPC Health
	// ctx1 := logger.WithLogger(ctx, e.Lg)
	// err = grpc_client.Check(ctx1, e.grpcClient, "")
	// if err != nil {
	// 	return fmt.Errorf("error check health: %w", err)
	// }

	e.Lg.Debug("Connecting to gRPC...", "url", cliCfg.Address)

	err = e.InsertFrom(ctx)
	return err
}

func (e *syncCommand) InsertFrom(ctx context.Context) (err error) {
	sErr := make([]error, 0)
	gcli := kbv1.NewStorAPIClient(e.grpcClient)

	fPath := filepath.Join(e.FileSource, "dep.json")
	err = e.insert(ctx, fPath, gcli, true)
	if err != nil {
		sErr = append(sErr, err)
	}

	fPath = filepath.Join(e.FileSource, "sotr.json")
	err = e.insert(ctx, fPath, gcli, false)
	if err != nil {
		sErr = append(sErr, err)
	}

	if len(sErr) > 0 {
		err = errors.Join(sErr...)
	}
	return
}

func (e *syncCommand) insert(ctx context.Context, path string, gcli kbv1.StorAPIClient, isDep bool) (err error) {
	var (
		s        string
		item     models.Item
		kbv1Item *kbv1.Item
	)

	if isDep {
		// item = &datasource.Dep{}
		item = &kbv1.Dep{}
		e.Lg.Debug("Load DepsResponse...", "dir", e.FileSource)
	} else {
		// item = &datasource.Sotr{}
		item = &kbv1.Sotr{}
		e.Lg.Debug("Load SotrsResponse...", "dir", e.FileSource)
	}

	f, err := os.Open(path)
	if err != nil {
		return
	}

	defer func() {
		err := f.Close()
		if err != nil {
			e.Lg.Error("defer close in insert()", "file", path, "err", err)
		}
		_, err = gcli.Flush(ctx, &emptypb.Empty{})
		if err != nil {
			e.Lg.Error("defer Flush in insert()", "file", path, "err", err)
		}
	}()

	frd := bufio.NewReader(f)

	for err != io.EOF {

		s, err = frd.ReadString('\n')

		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}

		err = protojson.Unmarshal([]byte(s), item)
		if err != nil {
			e.Lg.Error("insert: unmurshall json", "error", err, "json", s)
			continue
		}

		if isDep {
			kbv1Item = &kbv1.Item{Var: &kbv1.Item_Dep{Dep: item.(*kbv1.Dep)}}
		} else {
			kbv1Item = &kbv1.Item{Var: &kbv1.Item_Sotr{Sotr: item.(*kbv1.Sotr)}}
		}

		_, err = gcli.Save(ctx, kbv1Item)
		if err != nil {
			e.Lg.Error("insert: error save datasource.Dep", "err", err)
			continue
		}

		if isDep {
			e.DepsCounter.Add(1)
		} else {
			e.SotrCounter.Add(1)
		}
	}

	return
}
