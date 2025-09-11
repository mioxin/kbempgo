package cli

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/mioxin/kbempgo/internal/clientpool"
)

const ()

type employCommand struct {
	Glob     *Globals
	Workers  int          `name:"workers" short:"w" default:"3" env:"KB_WORKERS" help:"Number of workers"`
	Limit    int          `name:"limit" short:"l" default:"90" env:"KB_LIMIT" help:"Limit of data for get. If =0 then no limit."`
	RootRazd string       `name:"rootr" env:"KB_ROOT_RAZD" help:"Name of root section"`
	Lg       *slog.Logger `kong:"-"`
	Counter  atomic.Int32 `kong:"-"`
}

type ErrorMessage struct {
	Message string `json:"message"`
}

func (e *employCommand) Run(gl *Globals) error {

	e.Glob = gl
	gl.InitLog()
	e.Lg = gl.log.With("cmd", "employ")
	ctx := e.Glob.Context()
	razdCh := make(chan string)
	isData := make(chan struct{})

	//	clientsPool := clientpool.NewClientsPool(e.Workers)
	clientsPool := clientpool.NewClientPool(gl.Debug)

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
			w.Get(ctx, razdCh, clientsPool, isData, int32(e.Limit), &(e.Counter))
		}()

		// start dispatcher
		wgD.Add(1)
		go func() {
			defer wgD.Done()
			w.Dispatcher(ctx, razdCh, isData)
		}()
	}

	// Close Chanal after end of Dispatchers
	go func() {
		wgD.Wait()
		close(razdCh)
	}()

	// Start root section
	razdCh <- e.RootRazd

	wg.Wait()
	e.Lg.Debug("Stop wait group... Close depsCh.")
	return nil
}
