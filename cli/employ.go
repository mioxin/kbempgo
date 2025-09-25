package cli

import (
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

	"github.com/cespare/xxhash/v2"
	"github.com/mioxin/kbempgo/internal/utils"
)

const ()

type employCommand struct {
	Glob        *Globals
	Workers     int          `name:"workers" short:"w" default:"5" env:"KB_WORKERS" help:"Number of workers. Every worker run 3 goroutines."`
	Limit       int          `name:"limit" short:"l" default:"0" env:"KB_LIMIT" help:"Limit of data for get. If =0 then no limit."`
	RootRazd    string       `name:"rootr" env:"KB_ROOT_RAZD" help:"Name of root section"`
	Lg          *slog.Logger `kong:"-"`
	DepsCounter atomic.Int32 `kong:"-"`
	SotrCounter atomic.Int32 `kong:"-"`
}

type Task struct {
	Data string
	Num  int
}

func NewTask(data string) *Task {
	return &Task{data, 0}
}

type avatarInfo struct {
	ActualName string
	Num        int
	Size       int64
	Hash       string
}

func NewAvatarInfo(path string) (avatarInfo, error) {
	var (
		fileInfo os.FileInfo
		err      error
		num      int
	)
	sNum := ""

	fileInfo, err = os.Stat(path)
	if err != nil {
		return avatarInfo{}, err
	}

	slNum := strings.Split(strings.Split(fileInfo.Name(), ".")[0], " ")
	if len(slNum) > 1 {
		sNum = utils.FindBetween(slNum[1], "(", ")")
		if sNum != "" {
			num, err = strconv.Atoi(sNum)
			if err != nil {
				slog.Error("getFileCollection:", "err", err, "name", fileInfo.Name(), "sNum", sNum)
			}
		}
	}

	hash, err := hashFile(path)
	if err != nil {
		return avatarInfo{}, err
	}
	return avatarInfo{
		ActualName: fileInfo.Name(),
		Num:        num,
		Size:       fileInfo.Size(),
		Hash:       hash,
	}, nil

}

func (e *employCommand) Run(gl *Globals) error {

	e.Glob = gl
	gl.InitLog()
	e.Lg = gl.log.With("cmd", "employ")
	ctx := e.Glob.Context()
	razdCh := make(chan Task, 10)
	avatarCh := make(chan Task, 10)

	fileCollection, err := e.getFileCollection()
	if err != nil {
		return err
	}

	// init request workers
	pool := make([]*Worker, e.Workers)
	for i := range e.Workers {
		pool[i] = NewWorker(e.Glob, fmt.Sprintf("get-%d", i))
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
			w.Dispatcher(ctx, w.QueueDep, razdCh, w.isData)
		}()
		// start dispatcher Avatar
		wgD.Add(1)
		go func() {
			defer wgD.Done()
			w.Dispatcher(ctx, w.QueueAvatar, avatarCh, w.isDataA)
		}()
	}

	// Close Chanals if empty
	go func() {
		defer close(razdCh)
		defer close(avatarCh)
		timer := time.NewTicker(e.Glob.WaitDataTimeout)

		for {
			select {
			case <-e.Glob.ctx.Done():
			case <-timer.C:
				if len(razdCh) == 0 && len(avatarCh) == 0 {
					e.Lg.Info("Chanals razd&avatar is empty too long time: Timer cancel")
					e.Glob.cf()
					return
				}
			}
		}
	}()

	// Start root section
	razdCh <- *NewTask(e.RootRazd)

	wgD.Wait()
	wg.Wait()
	e.Lg.Debug("Stop wait group... Close depsCh.")
	e.Lg.Info("Collected.", "sotr", e.SotrCounter.Load(), "deps", e.DepsCounter.Load())
	return nil
}

func (e *employCommand) getFileCollection() (fColection map[string]avatarInfo, err error) {
	var (
		num             int
		key, sNum, hash string
	)
	fColection = make(map[string]avatarInfo, 1000)

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
			hash, err = hashFile(path)
			if err != nil {
				return err
			}
			fColection[key] = avatarInfo{
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

// hashFile calculate xxHash of file
func hashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed open file %s: %w", filePath, err)
	}
	defer file.Close()

	hasher := xxhash.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("ошибка чтения файла %s: %w", filePath, err)
	}

	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	return hash, nil
}
