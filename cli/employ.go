package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mioxin/kbempgo/internal/config"
	"github.com/mioxin/kbempgo/internal/utils"
	wrk "github.com/mioxin/kbempgo/internal/worker"
)

const ()

type employCommand struct {
	Glob        *config.Globals
	Workers     int          `name:"workers" short:"w" default:"5" env:"KB_WORKERS" help:"Number of workers. Every worker run 3 goroutines."`
	Limit       int          `name:"limit" short:"l" default:"0" env:"KB_LIMIT" help:"Limit of data for get. If =0 then no limit."`
	RootRazd    string       `name:"rootr" env:"KB_ROOT_RAZD" help:"Name of root section"`
	Lg          *slog.Logger `kong:"-"`
	DepsCounter atomic.Int32 `kong:"-"`
	SotrCounter atomic.Int32 `kong:"-"`
}

func (e *employCommand) Run(gl *config.Globals) error {
	e.Glob = gl
	gl.InitLog()
	e.Lg = gl.Log.With("cmd", "employ")
	ctx := e.Glob.Context()
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
	e.Lg.Debug("Stop wait group... Close depsCh.")
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
