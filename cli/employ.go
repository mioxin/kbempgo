package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mioxin/kbempgo/internal/storage"
	"github.com/mioxin/kbempgo/internal/utils"
	wrk "github.com/mioxin/kbempgo/internal/worker"
	"golang.org/x/sync/errgroup"
)

const ()

type dumpCommand struct {
	// Glob *config.Globals
	// KbUrl      string            `name:"scrape-url" placeholder:"URL" help:"Base Url"`
	// UrlRazd    string            `name:"scrape-razd" env:"KB_URL_RAZD" help:"Url of section"`
	// UrlSotr    string            `name:"scrape-sotr" env:"KB_URL_SOTR" help:"Url of employer"`
	// UrlFio     string            `name:"scrape-fio" env:"KB_URL_FIO" help:"Url of employer full nane"`
	// UrlMobile  string            `name:"scrape-mobil" env:"KB_URL_MOBIL" help:"Url of employer mobile"`
	// Avatars    string            `name:"scrape-avatars" env:"KB_AVATARS" help:"Directory for avatar images"`
	Workers  int    `name:"workers" short:"w" default:"5" env:"KB_WORKERS" help:"Number of workers. Every worker run 3 goroutines."`
	Limit    int    `name:"limit" short:"l" default:"0" env:"KB_LIMIT" help:"Limit of data for get. If =0 then no limit."`
	RootRazd string `name:"rootr" env:"KB_ROOT_RAZD" help:"Name of root section"`
	// FileSource string `name:"file_source" default:"" help:"Path includes dep.json and sotr.json for insert data from ones into storage"`
	// Grpc       gsrv.ServerConfig `embed:"" json:"grpc" prefix:"grpc-"`

	// grpcClient  *grpc.ClientConn `kong:"-"`
	Lg          *slog.Logger `kong:"-"`
	DepsCounter atomic.Int32 `kong:"-"`
	SotrCounter atomic.Int32 `kong:"-"`
}

func (e *dumpCommand) Run(cli *CLI) error {
	var err error

	e.Lg = cli.Log.With("cmd", "employ")

	ctx, cancel := context.WithTimeout(context.Background(), cli.OpTimeout)
	defer cancel()

	// ********************
	// dump data from url
	// ********************

	// open storage
	cli.Store, err = storage.NewStore(cli.StorageURL, e.Lg)
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
	fileCollection, err := e.getFileCollection(cli.Avatars)
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
	eg, ctxEg := errgroup.WithContext(ctx)

	razdCh := make(chan wrk.Task)
	avatarCh := make(chan wrk.Task)

	defer func() {
		close(razdCh)
		close(avatarCh)
	}()

	for _, w := range pool {
		eg.Go(func() error {
			return w.GetRazd(ctxEg, razdCh, int32(e.Limit), &(e.DepsCounter), &(e.SotrCounter))
		})

		eg.Go(func() error {
			return w.GetAvatar(ctxEg, avatarCh, int32(e.Limit), &(e.DepsCounter), &(e.SotrCounter), fileCollection)
		})

		// start dispatcher DepsResponse
		wgD.Add(1)

		go func() {
			defer wgD.Done()
			w.Dispatcher(ctx, w.Name, w.QueueDep, razdCh, w.IsData)
		}()

		// start dispatcher Avatar
		wgD.Add(1)

		go func() {
			defer wgD.Done()
			w.Dispatcher(ctx, "avatar", w.QueueAvatar, avatarCh, w.IsDataA)
		}()
	}

	// Stop workers
	go func() {
		isQueueEmpty := func() (ok bool) {
			ok = true
			for _, w := range pool {
				if w.QueueAvatar.Len() > 0 || w.QueueDep.Len() > 0 {
					ok = false
					break
				}
			}
			return
		}
		// waiting for worker's retrieve data start
		time.Sleep(5 * time.Second)
		timer := time.NewTicker(cli.HttpReqTimeout)

		for {
			select {
			case <-ctx.Done():
				e.Lg.Info("Main context concel done!")
				return

			case <-timer.C:
				if isQueueEmpty() {
					e.Lg.Info("Queues for razd&avatar is empty too long time: Timer cancel")
					cancel()
					return
				}
			}
		}
	}()

	// Start root section
	razdCh <- *wrk.NewTask(e.RootRazd)

	if err := eg.Wait(); err != nil {
		e.Lg.Error("Errgroup failed", "error", err)
	} else {
		e.Lg.Debug("All workers completed successfully")
	}

	cancel()
	wgD.Wait()

	e.Lg.Info("Collected.", "sotr", e.SotrCounter.Load(), "DepsResponse", e.DepsCounter.Load())
	return err
}

func (e *dumpCommand) getFileCollection(avatarsPath string) (fColection map[string]wrk.AvatarInfo, err error) {
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
