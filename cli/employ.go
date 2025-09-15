package cli

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const ()

type employCommand struct {
	Glob        *Globals
	Workers     int          `name:"workers" short:"w" default:"5" env:"KB_WORKERS" help:"Number of workers"`
	Limit       int          `name:"limit" short:"l" default:"0" env:"KB_LIMIT" help:"Limit of data for get. If =0 then no limit."`
	RootRazd    string       `name:"rootr" env:"KB_ROOT_RAZD" help:"Name of root section"`
	Lg          *slog.Logger `kong:"-"`
	DepsCounter atomic.Int32 `kong:"-"`
	SotrCounter atomic.Int32 `kong:"-"`
}

func (e *employCommand) Run(gl *Globals) error {

	e.Glob = gl
	gl.InitLog()
	e.Lg = gl.log.With("cmd", "employ")
	ctx := e.Glob.Context()
	razdCh := make(chan string, 10)

	// init request workers
	pool := make([]*Worker, e.Workers)
	for i := range e.Workers {
		pool[i] = NewWorker(e.Glob, fmt.Sprintf("get-%d", i))
	}

	// start request workers
	var wg sync.WaitGroup
	var wgD sync.WaitGroup
	for _, w := range pool {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Get(ctx, razdCh, int32(e.Limit), &(e.DepsCounter), &(e.SotrCounter))
		}()

		// start dispatcher
		wgD.Add(1)
		go func() {
			defer wgD.Done()
			w.Dispatcher(ctx, razdCh)
		}()
	}

	// Close Chanal if empty
	go func() {
		defer close(razdCh)
		timer := time.NewTicker(e.Glob.WaitDataTimeout)

		for {
			select {
			case <-e.Glob.ctx.Done():
			case <-timer.C:
				if len(razdCh) == 0 {
					e.Lg.Info("Chanal razd is empty too long time: Timer cancel")
					e.Glob.cf()
					return
				}
			}
		}
	}()

	// Start root section
	razdCh <- e.RootRazd

	wgD.Wait()
	wg.Wait()
	e.Lg.Debug("Stop wait group... Close depsCh.")
	e.Lg.Info("Collected.", "sotr", e.SotrCounter.Load(), "deps", e.DepsCounter.Load())
	return nil
}
