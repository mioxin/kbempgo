package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/config"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/mioxin/kbempgo/internal/utils"
	wrk "github.com/mioxin/kbempgo/internal/worker"
	"github.com/mioxin/kbempgo/pkg/grpc_client"
	gsrv "github.com/mioxin/kbempgo/pkg/grpc_server"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

const ()

type employCommand struct {
	Glob       *config.Globals
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

func (e *employCommand) Run(gl *config.Globals) error {
	var err error

	e.Glob = gl
	gl.InitLog()
	e.Lg = gl.Log.With("cmd", "employ")
	ctx := e.Glob.Context()

	// if FileSource setted then send data to storage
	if e.FileSource != "" {
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

		err = e.InsertFrom()
		return err
	}

	// scrape data from url
	razdCh := make(chan wrk.Task, 10)
	avatarCh := make(chan wrk.Task, 10)

	// fileCollection is map of existing avatar files info for define new avatars
	fileCollection, err := e.getFileCollection()
	if err != nil {
		return err
	}

	// init request workers
	pool := make([]*wrk.Worker, e.Workers)
	for i := range e.Workers {
		pool[i] = wrk.NewWorker(e.Glob, fmt.Sprintf("get-%d", i))
	}

	// start request workers
	var wg sync.WaitGroup

	var wgD sync.WaitGroup

	for _, w := range pool {
		wg.Add(2)

		go func() {
			defer wg.Done()

			w.GetRazd(ctx, razdCh, int32(e.Limit), &(e.DepsCounter), &(e.SotrCounter))
		}()

		go func() {
			defer wg.Done()

			w.GetAvatar(ctx, avatarCh, int32(e.Limit), &(e.DepsCounter), &(e.SotrCounter), fileCollection)
		}()

		// start dispatcher Deps
		wgD.Add(1)

		go func() {
			defer wgD.Done()

			w.Dispatcher(ctx, w.QueueDep, razdCh, w.IsData)
		}()

		// start dispatcher Avatar
		wgD.Add(1)

		go func() {
			defer wgD.Done()

			w.Dispatcher(ctx, w.QueueAvatar, avatarCh, w.IsDataA)
		}()
	}

	// Close Chanals if empty
	go func() {
		defer close(razdCh)
		defer close(avatarCh)

		timer := time.NewTicker(e.Glob.WaitDataTimeout)

		for {
			select {
			case <-e.Glob.Ctx.Done():
				return
			case <-timer.C:
				if len(razdCh) == 0 && len(avatarCh) == 0 {
					e.Lg.Info("Chanals razd&avatar is empty too long time: Timer cancel")
					e.Glob.Cf()

					return
				}
			}
		}
	}()

	// Start root section
	razdCh <- *wrk.NewTask(e.RootRazd)

	wgD.Wait()
	wg.Wait()
	e.Lg.Debug("Stop wait group... Close chanels.")
	e.Lg.Info("Collected.", "sotr", e.SotrCounter.Load(), "deps", e.DepsCounter.Load())

	return nil
}

func (e *employCommand) getFileCollection() (fColection map[string]wrk.AvatarInfo, err error) {
	var (
		num             int
		key, sNum, hash string
	)

	fColection = make(map[string]wrk.AvatarInfo, 1000)

	mywalkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		num = 1
		slNum := strings.Split(strings.Split(info.Name(), ".")[0], " ")

		key = slNum[0]
		if len(slNum) > 1 {
			sNum = utils.FindBetween(slNum[1], "(", ")")
			if sNum != "" {
				num, err = strconv.Atoi(sNum)
				if err != nil {
					e.Lg.Error("getFileCollection:", "err", err, "name", info.Name(), "sNum", sNum)
				}
			}
		}

		if avaInf, ok := fColection[key]; !ok || avaInf.Num < num {
			hash, err = wrk.HashFile(path)
			if err != nil {
				return err
			}

			fColection[key] = wrk.AvatarInfo{
				ActualName: info.Name(),
				Num:        num,
				Size:       info.Size(),
				Hash:       hash,
			}
		}

		return nil
	}
	if err = filepath.Walk(e.Glob.Avatars, mywalkFunc); err != nil {
		err = fmt.Errorf("get avatar colection: %w", err)
	}

	return fColection, err
}

type ItemKbv interface {
	Conv2Kbv() *kbv1.Item
}

func insert[T ItemKbv](ctx context.Context, path string, gcli kbv1.StorClient) (err error) {
	var (
		s string
	)
	item := new(T)

	f, err := os.Open(path)
	if err != nil {
		return
	}

	defer func() {
		err := f.Close()
		if err != nil {
			slog.Error("defer close in insert()", "file", path, "err", err)
		}
		_, err = gcli.Flush(ctx, &emptypb.Empty{})
		if err != nil {
			slog.Error("defer Flush in insert()", "file", path, "err", err)
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

		err = json.Unmarshal([]byte(s), item)
		if err != nil {
			slog.Error("insert: unmurshall json", "error", err, "json", s)
			continue
		}

		switch it := any(item).(type) {
		case *models.Dep:
			_, err = gcli.Save(ctx, it.Conv2Kbv())
			if err != nil {
				slog.Error("insert: error save models.Dep", "err", err)
			}
		case *models.Sotr:
			_, err = gcli.Save(ctx, it.Conv2Kbv())
			if err != nil {
				slog.Error("insert: error save models.Sotr", "err", err)
			}
		default:
			slog.Error("insert: error cast item to models.Dep or models.Sotr", "json", s)
		}
	}
	return
}

func (e *employCommand) InsertFrom() (err error) {

	gcli := kbv1.NewStorClient(e.grpcClient)

	fPath := filepath.Join(e.FileSource, "dep.json")
	err = insert[models.Dep](e.Glob.Ctx, fPath, gcli)

	fPath = filepath.Join(e.FileSource, "sotr.json")
	err1 := insert[models.Sotr](e.Glob.Ctx, fPath, gcli)

	err = errors.Join(err, err1)

	return
}
