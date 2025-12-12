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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/mioxin/kbempgo/internal/storage"
	"github.com/mioxin/kbempgo/internal/utils"
	wrk "github.com/mioxin/kbempgo/internal/worker"
	"github.com/mioxin/kbempgo/pkg/grpc_client"
	gsrv "github.com/mioxin/kbempgo/pkg/grpc_server"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
)

const ()

type employCommand struct {
	// Glob *config.Globals
	// KbUrl      string            `name:"scrape-url" placeholder:"URL" help:"Base Url"`
	// UrlRazd    string            `name:"scrape-razd" env:"KB_URL_RAZD" help:"Url of section"`
	// UrlSotr    string            `name:"scrape-sotr" env:"KB_URL_SOTR" help:"Url of employer"`
	// UrlFio     string            `name:"scrape-fio" env:"KB_URL_FIO" help:"Url of employer full nane"`
	// UrlMobile  string            `name:"scrape-mobil" env:"KB_URL_MOBIL" help:"Url of employer mobile"`
	// Avatars    string            `name:"scrape-avatars" env:"KB_AVATARS" help:"Directory for avatar images"`
	Workers    int               `name:"workers" short:"w" default:"5" env:"KB_WORKERS" help:"Number of workers. Every worker run 3 goroutines."`
	Limit      int               `name:"limit" short:"l" default:"0" env:"KB_LIMIT" help:"Limit of data for get. If =0 then no limit."`
	RootRazd   string            `name:"rootr" env:"KB_ROOT_RAZD" help:"Name of root section"`
	FileSource string            `name:"file_source" default:"" help:"Path includes dep.json and sotr.json for insert data from ones into storage"`
	Grpc       gsrv.ServerConfig `embed:"" json:"grpc" prefix:"grpc-"`

	grpcClient          *grpc.ClientConn `kong:"-"`
	Lg                  *slog.Logger     `kong:"-"`
	DepsResponseCounter atomic.Int32     `kong:"-"`
	SotrCounter         atomic.Int32     `kong:"-"`
}

func (e *employCommand) Run(cli *CLI) error {
	var err error

	e.Lg = cli.Log.With("cmd", "employ")

	ctx, cancel := context.WithTimeout(context.Background(), cli.OpTimeout)
	defer cancel()

	// ****************************************
	// if FileSource setted then load data from source to storage
	// ****************************************
	if e.FileSource != "" {
		err := e.LoadDataToStor(ctx)
		e.Lg.Info("Loaded from", "dir", e.FileSource, "dep_count", e.DepsResponseCounter.Load(), "sotr_count", e.SotrCounter.Load())
		return err
	}
	// ********************
	// scrape data from url
	// ********************

	// open storage
	cli.Store, err = storage.NewStore(cli.DbUrl, e.Lg)
	if err != nil {
		return fmt.Errorf("create storage %w", err)
	}

	defer func() {
		cli.Log.Info("Close storage")

		if err := cli.Store.Close(); err != nil {
			cli.Log.Error("close storage", "err", err)
		}
	}()

	// fileCollection is map of existing avatar files info for define new avatars
	fileCollection, err := e.getFileCollection(cli.Config.Avatars)
	if err != nil {
		return err
	}

	// init request workers
	pool := make([]*wrk.Worker, e.Workers)
	for i := range e.Workers {
		pool[i] = wrk.NewWorker(&cli.Config, fmt.Sprintf("get-%d", i), cli.Debug, e.Lg)
	}

	// start request workers
	// var wg sync.WaitGroup
	var wgD sync.WaitGroup
	eg, ctxErr := errgroup.WithContext(ctx)

	razdCh := make(chan wrk.Task, 10)
	avatarCh := make(chan wrk.Task, 10)

	defer func() {
		close(razdCh)
		close(avatarCh)
	}()

	for _, w := range pool {
		eg.Go(func() error {
			return w.GetRazd(ctxErr, razdCh, int32(e.Limit), &(e.DepsResponseCounter), &(e.SotrCounter))
		})

		eg.Go(func() error {
			return w.GetAvatar(ctxErr, avatarCh, int32(e.Limit), &(e.DepsResponseCounter), &(e.SotrCounter), fileCollection)
		})

		// start dispatcher DepsResponse
		wgD.Add(1)

		go func() {
			defer wgD.Done()
			w.Dispatcher(ctx, "razd", w.QueueDep, razdCh, w.IsData)
		}()

		// start dispatcher Avatar
		wgD.Add(1)

		go func() {
			defer wgD.Done()
			w.Dispatcher(ctx, "avatar", w.QueueAvatar, avatarCh, w.IsDataA)
		}()
	}

	// Close Chanals if empty
	go func() {
		timer := time.NewTicker(cli.WaitDataTimeout)

		for {
			select {
			case <-ctxErr.Done():
				eg.Wait()
				cancel()
				return

			case <-ctx.Done():
				return

			case <-timer.C:
				if len(razdCh) == 0 && len(avatarCh) == 0 {
					e.Lg.Info("Chanals razd&avatar is empty too long time: Timer cancel")
					cancel()
					return
				}
			}
		}
	}()

	// Start root section
	razdCh <- *wrk.NewTask(e.RootRazd)

	err = eg.Wait()
	wgD.Wait()

	e.Lg.Debug("Stop wait group... Close chanels.")
	e.Lg.Info("Collected.", "sotr", e.SotrCounter.Load(), "DepsResponse", e.DepsResponseCounter.Load())

	return err
}

func (e *employCommand) getFileCollection(avatarsPath string) (fColection map[string]wrk.AvatarInfo, err error) {
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
	if err = filepath.Walk(avatarsPath, mywalkFunc); err != nil {
		err = fmt.Errorf("get avatar colection: %w", err)
	}

	return fColection, err
}

func (e *employCommand) LoadDataToStor(ctx context.Context) (err error) {
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

func (e *employCommand) InsertFrom(ctx context.Context) (err error) {
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

func (e *employCommand) insert(ctx context.Context, path string, gcli kbv1.StorAPIClient, isDep bool) (err error) {
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
			e.DepsResponseCounter.Add(1)
		} else {
			e.SotrCounter.Add(1)
		}
	}

	return
}
