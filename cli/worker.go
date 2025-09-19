package cli

import (
	"container/list"
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	mu          sync.Mutex
	Name        string
	QueueDep    *list.List
	QueueAvatar *list.List
	Gl          *Globals
	isData      chan struct{}
	isDataA     chan struct{}
	Lg          *slog.Logger
}

func NewWorker(gl *Globals, name string) *Worker {
	lg := gl.log.With("worker", name)
	isData := make(chan struct{})
	isDataA := make(chan struct{})

	return &Worker{Name: name,
		QueueDep:    list.New(),
		QueueAvatar: list.New(),
		Gl:          gl,
		Lg:          lg,
		isData:      isData,
		isDataA:     isDataA,
	}
}

func (w *Worker) GetRazd(ctx context.Context, in <-chan string, limit int32, depsCount *atomic.Int32, sotrCount *atomic.Int32) {
	var errMsg ErrorMessage

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
					w.QueueDep.PushBack(d.Idr)
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
		w.Lg.Debug("Worker Len of QueueDep:", "len", w.QueueDep.Len())

	}
}

// Dispatcher wolking accross worker queues for forwarding idr/avatar to worker input chanal
func (w *Worker) Dispatcher(ctx context.Context, out, outA chan<- string) {
	var (
		count, lenQ int
		s           string
	)

	send2chan := func(q *list.List, outCh chan<- string) (ok bool) {

		ok = true
		w.mu.Lock()
		lenQ = q.Len()
		w.mu.Unlock()

		if lenQ == 0 {
			w.Lg.Info("Dispatcher error isData when QueueDep len =0")
			return
		}
		w.Lg.Debug("Dispatcher. Get new data from QueueDep...")
		// get data from queue
		w.mu.Lock()
		s = q.Remove(q.Front()).(string)
		w.mu.Unlock()

		// send data to out chanal (razdCh)
		select {
		case outCh <- s:
			count++
			w.Lg.Info("Dispatcher: Get from queue", "count", count, "data", s)
		case <-ctx.Done():
			w.Lg.Info("Dispatcher: " + ctx.Err().Error())
			return false
		}

		return
	}

	for {
		// if the dispatcher faster a workers then isData indicate new data in the queue
		// if w.QueueDep.Len() == 0 {
		w.Lg.Debug("Dispatcher. Getting data from Queues...")
		select {
		case _, ok := <-w.isData:
			if !ok {
				return
			}
			if !send2chan(w.QueueDep, out) {
				return
			}

		case _, ok := <-w.isDataA:
			if !ok {
				return
			}
			if !send2chan(w.QueueAvatar, outA) {
				return
			}

		case <-ctx.Done():
			w.Lg.Info("Dispatcher: cancel" + ctx.Err().Error())
			return
		}
		// }
	}
}

// define the item as a deps or an employee and save one
func (w *Worker) PrepareItem(cli *req.Client, item Item) error {
	dep := item.(*models.Dep)
	dep.Text = html.UnescapeString(dep.Text)
	if item.IsSotr() {

		sotr := utils.ParseSotr(dep.Text)

		// send url Avatar image to queue for download
		w.mu.Lock()
		w.QueueAvatar.PushBack(sotr.Avatar)
		w.mu.Unlock()

		select {
		case w.isDataA <- struct{}{}:
		default:
		}

		sotr.Children = dep.Children
		sotr.Idr = dep.Idr
		sotr.ParentId = dep.Parent

		// define the middle name of employee
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
		err = fmt.Errorf("get middle name: error handling %w", err)
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

func (w *Worker) GetAvatar(ctx context.Context, in <-chan string, limit int32, depsCount *atomic.Int32, sotrCount *atomic.Int32) {
	var errMsg ErrorMessage

	w.Lg.Info("Worker avatar: Start Getting...")

	var cli *req.Client
	defer func() {
		w.Gl.clientsPool.Push(cli)
		w.Lg.Info("Worker avatar: END")
	}()

	// get client from pool
	cli = w.Gl.clientsPool.Get()
	cli.SetBaseURL(w.Gl.KbUrl).
		SetTimeout(w.Gl.HttpReqTimeout)

	callback := func(info req.DownloadInfo) {
		if info.Response.Response != nil {
			fmt.Printf("downloaded %.2f%% (%s)\n", float64(info.DownloadedSize)/float64(info.Response.ContentLength)*100.0, info.Response.Request.URL.String())
		}
	}

	for ava := range in {

		cnt := depsCount.Load() + sotrCount.Load()
		if limit > 0 && cnt > limit {
			w.Lg.Info("Worker avatar: Count limited", "count", cnt)
			w.Gl.Done()
			return
		}

		w.Lg.Debug("Worker avatar:", "avatar", ava)

		filename := filepath.Join(w.Gl.Avatars, ava)
		f, err := os.Stat(filename)

		if err == nil && !f.IsDir() {
			r := cli.Head(ava).
				Do()

			if r.Err != nil {
				fmt.Println(r.Err.Error(), ava)
			}
			if r.ContentLength == f.Size() {
				w.Lg.Debug("Worker avatar: Skip existing >>>", "file", ava)
				continue
			}
		}

		fl, _ := strings.CutPrefix(ava, "/")

		resp, err := cli.R().
			SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
			SetOutputFile(fl).
			SetDownloadCallback(callback).
			Get(ava)
		if err != nil { // Error handling.
			w.Lg.Error("Worker avatar:", "error handling", err)
		}

		if resp.IsErrorState() { // Status code >= 400.
			w.Lg.Error("Worker avatar:", "err", errMsg.Message) // Record error message returned.
		}

		if resp.IsSuccessState() { // Status code is between 200 and 299.
			w.Lg.Info("Worker avatar: downloaded", "avatar", ava, "syze", resp.ContentLength)
		}
		w.Lg.Debug("Worker avatar: Len of QueueAvatar", "len", w.QueueAvatar.Len())
	}
}
