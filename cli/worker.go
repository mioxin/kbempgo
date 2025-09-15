package cli

import (
	"container/list"
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/imroc/req/v3"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/mioxin/kbempgo/internal/utils"
)

type ErrorMessage struct {
	Message string `json:"message"`
}

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
	mu     sync.Mutex
	Name   string
	Queue  *list.List
	Gl     *Globals
	isData chan struct{}
	Lg     *slog.Logger
}

func NewWorker(gl *Globals, name string) *Worker {
	lg := gl.log.With("worker", name)
	isData := make(chan struct{})

	return &Worker{Name: name,
		Queue:  list.New(),
		Gl:     gl,
		Lg:     lg,
		isData: isData,
	}
}

func (w *Worker) Get(ctx context.Context, in <-chan string, limit int32, depsCount *atomic.Int32, sotrCount *atomic.Int32) {
	var (
		errMsg ErrorMessage
	)

	w.Lg.Info("Worker: Start Getting...")

	var cli *req.Client
	defer func() {
		w.Gl.clientsPool.Push(cli)
		w.Lg.Info("Worker: END")
	}()

	// get client from pool
	cli = w.Gl.clientsPool.Get()
	cli.SetBaseURL(w.Gl.KbUrl).
		SetTimeout(w.Gl.HttpReqTimeout)

	for r := range in {

		cnt := depsCount.Load() + sotrCount.Load()
		if limit > 0 && cnt > limit {
			w.Lg.Info("Worker: Count limited", "count", cnt)
			w.Gl.Done()
			return
		}

		w.Lg.Debug("Worker:", "dep", r)

		deps := make([]*models.Dep, 0)
		resp, err := cli.R().
			SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
			EnableDump().            // Enable dump at request level, only print dump content if there is an error or some unknown situation occurs to help troubleshoot.
			SetSuccessResult(&deps). // Unmarshal response body into userInfo automatically if status code is between 200 and 299.
			SetContext(ctx).
			Get(w.Gl.UrlRazd + r)

		select {
		case <-ctx.Done():
			w.Lg.Info("Worker:", "err", ctx.Err().Error())
			return
		default:
		}

		if err != nil { // Error handling.
			w.Lg.Error("Worker:", "error handling", err)
			w.Lg.Debug("Worker:", "resp dump", resp.Dump()) // Record raw content when error occurs.
		}

		if resp.IsErrorState() { // Status code >= 400.
			w.Lg.Error("Worker:", "err", errMsg.Message) // Record error message returned.
			break
		}

		if resp.IsSuccessState() { // Status code is between 200 and 299.

			rBytes := []byte{}
			for _, r := range deps {
				rBytes = append(rBytes, []byte(r.Idr)...)
				rBytes = append(rBytes, []byte("; ")...)
			}
			w.Lg.Debug("Worker responce:", "razd", string(rBytes), "deps length", len(deps), "time", resp.TotalTime())
			if string(rBytes) == "" {
				w.Lg.Warn("Worker: Empty response", "req dep", r, "resp", string(resp.Bytes()), "time", resp.TotalTime())
			}

			for _, d := range deps {

				if !d.IsSotr() {
					w.mu.Lock()
					w.Queue.PushBack(d.Idr)
					w.mu.Unlock()

					depsCount.Add(1)

					select {
					case w.isData <- struct{}{}:
					default:
					}

				} else {
					sotrCount.Add(1)
				}

				err := w.PrepareItem(cli, d)
				if err != nil {
					w.Lg.Error("Worker: Prepare dep", "err", err, "dep", d)
				}

			}
		}
		w.Lg.Debug("Worker Len of Queue:", "len", w.Queue.Len())

	}
}

// Dispatcher wolking accross worker queues for forwarding idr/avatar to worker input chanal
func (w *Worker) Dispatcher(ctx context.Context, out chan<- string) {

	count := 0
	s := ""

	for {
		// if the dispatcher faster a workers then isData indicate new data in the queue
		if w.Queue.Len() == 0 {
			w.Lg.Debug("Dispatcher. Waiting data from Queue...")
			select {
			case _, ok := <-w.isData:
				if !ok {
					return
				}
				if w.Queue.Len() == 0 {
					w.Lg.Info("Dispatcher error isData when Queue len =0")
					continue
				}

			case <-ctx.Done():
				w.Lg.Info("Dispatcher: cancel" + ctx.Err().Error())
				return
			}
		}
		w.Lg.Debug("Dispatcher. Get new data from Queue...")
		// get data from queue
		w.mu.Lock()
		s = w.Queue.Remove(w.Queue.Front()).(string)
		w.mu.Unlock()

		// send data to out chanal (razdCh)
		select {
		case out <- s:
			count++
			w.Lg.Info("Dispatcher: Get from queue", "dep count", count, "razd", s)
		case <-ctx.Done():
			w.Lg.Info("Dispatcher: " + ctx.Err().Error())
			return
		}
	}
}

func (w *Worker) PrepareItem(cli *req.Client, item Item) error {
	dep := item.(*models.Dep)
	dep.Text = html.UnescapeString(dep.Text)
	if item.IsSotr() {

		sotr := utils.ParseSotr(dep.Text)
		sotr.Children = dep.Children
		sotr.Idr = dep.Idr
		sotr.ParentId = dep.Parent

		text, err := w.getMiddleName(w.Gl.ctx, cli, sotr.Name)
		if err != nil {
			w.Lg.Error(err.Error())
		} else {
			sotr.MidName = utils.ParseMidName(sotr, html.UnescapeString(text))
		}

		if sotr.MidName == "" {
			w.Lg.Warn("Middle name not found", "short name", sotr.Name)
		}

		item = sotr
	}
	err := w.Gl.store.Save(item)
	return err
}

func (w *Worker) getMiddleName(ctx context.Context, cli *req.Client, shortName string) (string, error) {
	var (
		errMsg ErrorMessage
		body   string // []byte
	)

	resp, err := cli.R().
		SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
		Get(w.Gl.UrlFio + url.PathEscape(shortName))

	select {
	case <-ctx.Done():
		w.Lg.Info(ctx.Err().Error())
		return (body), ctx.Err()
	default:
	}

	if err != nil { // Error handling.
		err = fmt.Errorf("Get Middle name: error handling %w", err)
		w.Lg.Debug("Get Middle name error: raw content", "resp dump", resp.Dump()) // Record raw content when error occurs.
	}

	if resp.IsErrorState() { // Status code >= 400.
		w.Lg.Error(errMsg.Message) // Record error message returned.
	}

	if resp.IsSuccessState() { // Status code is between 200 and 299.
		body = resp.String()
		// w.Lg.Debug("Get middle name text", "size", n, "short name", shortName)
	}
	return (body), err
}
