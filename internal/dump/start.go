package dump

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mioxin/kbempgo/internal/models"
	"github.com/mioxin/kbempgo/internal/utils"
	"github.com/mioxin/kbempgo/internal/worker"
	"golang.org/x/sync/errgroup"
)

var (
	DepsCounter atomic.Int32
	SotrCounter atomic.Int32
)

func StartDump(ctx context.Context, cancel context.CancelFunc, cfg *Config) <-chan (models.Item) {
	outCh := make(chan (models.Item), 1000)
	//defer close(outCh)

	// *****************************
	// init request workers

	// fileCollection is map of existing avatar files info for define new avatars
	fileCollection, err := getFileCollection(cfg.Avatars, cfg.Lg)
	if err != nil {
		return outCh
	}

	pool := make([]*worker.Worker, cfg.Workers)
	for i := range cfg.Workers {
		pool[i] = worker.NewWorker(&cfg.Config, fmt.Sprintf("get-%d", i), cfg.Debug, cfg.Lg)
	}

	// start request workers
	go func() {
		// var wgD sync.WaitGroup
		ctxw, cancelw := context.WithCancel(ctx)
		eg, ctxEg := errgroup.WithContext(ctxw)

		razdCh := make(chan worker.Task)
		avatarCh := make(chan worker.Task)

		defer close(razdCh)
		defer close(avatarCh)

		for _, w := range pool {
			eg.Go(func() error { return w.GetRazd(ctxEg, razdCh, outCh, int32(cfg.Limit), &(DepsCounter), &(SotrCounter)) })
			eg.Go(func() error {
				return w.GetAvatar(ctxEg, avatarCh, int32(cfg.Limit), &(DepsCounter), &(SotrCounter), fileCollection)
			})

			// start dispatcher DepsResponse
			go func() {
				w.Dispatcher(ctx, w.Name, w.QueueDep, razdCh) //, w.IsData)
			}()

			// start dispatcher Avatar
			go func() {
				w.Dispatcher(ctx, "avatar", w.QueueAvatar, avatarCh) //, w.IsDataA)
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
			timer := time.NewTicker(cfg.HttpReqTimeout)

			for {
				select {
				case <-ctx.Done():
					cfg.Lg.Info("Main context concel done!")
					return

				case <-timer.C:
					if isQueueEmpty() {
						cfg.Lg.Info("Queues for razd&avatar is empty too long time: Timer cancel")
						cancelw()
						return
					}
				}
			}
		}()

		// Start root section
		razdCh <- *worker.NewTask(cfg.RootRazd)

		if err := eg.Wait(); err != nil {
			cfg.Lg.Error("Errgroup failed", "error", err)
		} else {
			cfg.Lg.Debug("All workers completed successfully")
		}

		cancel()
		// wgD.Wait()
		// ****************************************
	}()

	return outCh
}

// Collect avatars that exits for avoid a double downloading
func getFileCollection(avatarsPath string, lg *slog.Logger) (fColection map[string]worker.AvatarInfo, err error) {
	var (
		num             int
		key, sNum, hash string
	)

	fColection = make(map[string]worker.AvatarInfo, 1000)

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
					lg.Error("getFileCollection:", "err", err, "name", info.Name(), "sNum", sNum)
				}
			}
		}

		if avaInf, ok := fColection[key]; !ok || avaInf.Num < num {
			hash, err = worker.HashFile(path)
			if err != nil {
				return err
			}

			fColection[key] = worker.AvatarInfo{
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
