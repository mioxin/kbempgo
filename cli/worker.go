package cli

import (
	"container/list"
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/imroc/req/v3"
	"github.com/mioxin/kbempgo/internal/models"
)

type HttpClientPool interface {
	//  chanal implementation
	//	Get(int) *req.Client
	//	Push(*req.Client) error

	//  sync.Pool implementation
	Get() *req.Client
	Push(*req.Client)
}

type Item interface {
	IsSotr() bool
}

type Worker struct {
	mu    sync.Mutex
	Name  string
	Queue *list.List
	Gl    *Globals
	Lg    *slog.Logger
}

func NewWorker(gl *Globals, name string) *Worker {
	lg := gl.log.With("worker", name)

	return &Worker{Name: name,
		Queue: list.New(),
		Gl:    gl,
		Lg:    lg,
	}
}

func (w *Worker) Get(ctx context.Context, in <-chan string, cliPool HttpClientPool, isData chan struct{}, limit int32, count *atomic.Int32) {
	var (
		errMsg ErrorMessage
	)

	w.Lg.Info("Start Getting...")

	var cli *req.Client
	defer func() {
		cliPool.Push(cli)
		w.Lg.Info("END")
	}()

	// get client from pool
	cli = cliPool.Get()
	cli.SetBaseURL(w.Gl.KbUrl)

	for r := range in {
		cnt := count.Load()
		if limit > 0 && cnt > limit {
			w.Lg.Info("Count limited", "count", cnt)
			w.Gl.Done()
			return
		}

		ctx1, _ := context.WithTimeout(ctx, w.Gl.WaitDataTimeout)
		w.Lg.Debug(r)
		deps := make([]*models.Dep, 0)
		resp, err := cli.R().
			SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
			EnableDump().            // Enable dump at request level, only print dump content if there is an error or some unknown situation occurs to help troubleshoot.
			SetSuccessResult(&deps). // Unmarshal response body into userInfo automatically if status code is between 200 and 299.
			SetContext(ctx1).
			Get(w.Gl.UrlRazd + r)

		select {
		case <-ctx.Done():
			w.Lg.Info(ctx1.Err().Error())
			return
		default:
		}

		if err != nil { // Error handling.
			w.Lg.Error("error handling", "err", err)
			w.Lg.Debug("raw content:", "resp dump", resp.Dump()) // Record raw content when error occurs.
		}

		if resp.IsErrorState() { // Status code >= 400.
			w.Lg.Error(errMsg.Message) // Record error message returned.
			break
		}

		if resp.IsSuccessState() { // Status code is between 200 and 299.
			w.Lg.Debug("responce:", "razd", deps)
			for _, d := range deps {

				err := w.PrepareItem(d)
				if err != nil {
					w.Lg.Error("Prepare dep", "dep", d)
				}

				w.mu.Lock()
				w.Queue.PushBack(d.Idr)
				w.mu.Unlock()

				count.Add(1)

				select {
				case isData <- struct{}{}:
				default:
				}
			}

		}
	}
}

// Dispatcher wolking accross worker queues for forwarding idr to worker input chanal
func (w *Worker) Dispatcher(ctx context.Context, out chan<- string, isData <-chan struct{}) {

	count := 0
	s := ""

	ctx, cancel := context.WithTimeout(ctx, w.Gl.WaitDataTimeout)
	defer cancel()
	for {
		select {
		// if the dispatcher faster a workers then isData indicate new data in the queue
		case <-isData:

		case <-ctx.Done():
			w.Lg.Info("Dispatcher: waiting data" + ctx.Err().Error())
			return
		}

		// get data from queue
		w.mu.Lock()
		if w.Queue.Len() > 0 {
			s = w.Queue.Remove(w.Queue.Front()).(string)
			w.mu.Unlock()

		} else {
			w.mu.Unlock()
			continue
		}

		// send data to out chanal (razdCh)
		select {
		case out <- s:
			count++
			w.Lg.Info("Dispatcher: Get from pool", "count", count, "razd", s)
		case <-ctx.Done():
			w.Lg.Info("Dispatcher: " + ctx.Err().Error())
			return
		}
	}
}

func (w *Worker) PrepareItem(item Item) error {

	if item.IsSotr() {

	}
	err := w.Gl.store.Save(item)
	return err
}
