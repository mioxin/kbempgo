package worker

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
	"time"

	"github.com/imroc/req/v3"
	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/config"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/mioxin/kbempgo/internal/utils"
)

type Task struct {
	Data string
	Num  int
}

func NewTask(data string) *Task {
	return &Task{data, 0}
}

type ErrorMessage struct {
	Message string `json:"message"`
}

// TryLimit is retry get razd if empty one
const TryLimit int = 3

type Item interface {
	GetChildren() bool
}

type Worker struct {
	mu          sync.Mutex
	Name        string
	QueueDep    *list.List
	QueueAvatar *list.List
	Gl          *config.Globals
	IsData      chan struct{}
	IsDataA     chan struct{}
	Lg          *slog.Logger
}

func NewWorker(gl *config.Globals, name string) *Worker {
	lg := gl.Log.With("worker", name)
	isData := make(chan struct{}, 1500)
	IsDataA := make(chan struct{}, 1500)

	return &Worker{Name: name,
		QueueDep:    list.New(),
		QueueAvatar: list.New(),
		Gl:          gl,
		Lg:          lg,
		IsData:      isData,
		IsDataA:     IsDataA,
	}
}

// GetRazd ...
func (w *Worker) GetRazd(ctx context.Context, in chan Task, limit int32, depsCount *atomic.Int32, sotrCount *atomic.Int32) {
	var (
		errMsg ErrorMessage
	)

	w.Lg.Info("Worker: Start Getting...")

	var cli *req.Client

	defer func() {
		// w.Gl.ClientsPool.Push(cli)
		w.Lg.Info("Worker: END")
	}()

	// get client from pool
	// cli = w.Gl.ClientsPool.Get()
	cli = w.Gl.ClientsPool

	cli.SetBaseURL(w.Gl.KbUrl).
		SetTimeout(w.Gl.HttpReqTimeout).
		SetLogger(&ReqLogger{Logger: *w.Lg})

	for r := range in {
		cnt := depsCount.Load() + sotrCount.Load()
		if limit > 0 && cnt > limit {
			w.Lg.Info("Worker: Count limited", "count", cnt)
			w.Gl.Done()

			return
		}

		if r.Num > TryLimit {
			w.Lg.Warn("Worker: Out of try limit", "try", r.Num, "req_dep", r.Data)
			break
		}

		w.Lg.Debug("Worker:", "dep", r.Data, "try", r.Num)

		deps := make([]*models.Dep, 0)

		// retry if successful req but empty deps
		resp, err := cli.R().
			SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
			EnableDump().            // Enable dump at request level, only print dump content if there is an error or some unknown situation occurs to help troubleshoot.
			SetSuccessResult(&deps). // Unmarshal response body into userInfo automatically if status code is between 200 and 299.
			// SetContext(ctx).
			Get(w.Gl.UrlRazd + r.Data)

		select {
		case <-ctx.Done():
			w.Lg.Info("Worker:", "err", ctx.Err().Error())
			return
		default:
		}

		if err != nil { // Error handling.
			w.Lg.Error("Worker:", "error handling", err)
			w.Lg.Debug("Worker:", "resp dump", resp.Dump()) // Record raw content when error occurs.

			break
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

			w.Lg.Debug("Worker: responce:", "razd", string(rBytes), "deps_length", len(deps), "time", resp.TotalTime())

			// retry get razd
			if string(rBytes) == "" { // && resp.TotalTime() > 4*time.Second {
				w.Lg.Warn("Worker: Empty response ", "try", r.Num, "req_dep", r.Data, "resp", resp.Dump(), "time", resp.TotalTime())

				in <- Task{r.Data, r.Num + 1}

				continue
			}

			for _, d := range deps {
				if !d.GetChildren() {
					w.mu.Lock()
					w.QueueDep.PushBack(d.Idr)
					w.mu.Unlock()

					depsCount.Add(1)

					select {
					case w.IsData <- struct{}{}:
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

		w.mu.Lock()
		lenQ := w.QueueDep.Len()
		w.mu.Unlock()

		w.Lg.Debug("Worker Len of QueueDep:", "len", lenQ)
	}
}

// Dispatcher wolking accross worker queues for forwarding idr/avatar to worker input chanal
func (w *Worker) Dispatcher(ctx context.Context, queue *list.List, out chan<- Task, isData <-chan struct{}) {
	var (
		count, lenQ int
		s           string
	)

	send2chan := func(q *list.List, outCh chan<- Task) (ok bool) {
		ok = true

		w.mu.Lock()

		lenQ = q.Len()

		w.mu.Unlock()

		if lenQ == 0 {
			w.Lg.Info("Dispatcher error isData when Queue len =0")
			return
		}
		// w.Lg.Debug("Dispatcher. Get new data from Queue...")
		// get data from queue
		w.mu.Lock()
		s = q.Remove(q.Front()).(string)
		w.mu.Unlock()

		// send data to out chanal (razdCh)
		select {
		case outCh <- *NewTask(s):
			count++
		case <-ctx.Done():
			w.mu.Lock()
			lenQ = q.Len()
			w.mu.Unlock()

			w.Lg.Info("Dispatcher: general cancel:", "err", ctx.Err().Error(), "QueueLen", lenQ) // , "DepsQueueLen", lenQD)
			ok = false

			return
		}

		return
	}

	for {
		// if the dispatcher faster a workers then isData indicate new data in the queue
		// if w.QueueDep.Len() == 0 {
		w.Lg.Debug("Dispatcher. Getting data from Queue...")

		timeStart := time.Now()

		select {
		case _, ok := <-isData:
			if !ok {
				return
			}

			if !send2chan(queue, out) {
				return
			}

			w.mu.Lock()
			lenQ := queue.Len()
			w.mu.Unlock()

			w.Lg.Info("Dispatcher: Get from queue", "count", count, "data", s, "QueueLen", lenQ, "time", time.Since(timeStart))

		case <-ctx.Done():
			w.mu.Lock()
			lenQ := queue.Len()
			w.mu.Unlock()

			w.Lg.Debug("Dispatcher: general cancel in loop:", "err", ctx.Err().Error(), "QueueLen", lenQ, "IsData", len(isData)) // , "DepsQueueLen", lenQD, "DepsIsData", len(w.isData))

			return
		}
		// }
	}
}

// PrepareItem define the item as a deps or an employee and save one
func (w *Worker) PrepareItem(cli *req.Client, item Item) error {
	dep := item.(*kbv1.Dep)
	dep.Text = html.UnescapeString(dep.Text)

	if item.GetChildren() {
		sotr := utils.ParseSotr(dep.Text)

		// send url Avatar image to queue for download
		w.mu.Lock()
		w.QueueAvatar.PushBack(sotr.Avatar)
		w.mu.Unlock()

		select {
		case w.IsDataA <- struct{}{}:
		default:
		}

		sotr.Children = dep.Children
		sotr.Idr = dep.Idr
		sotr.ParentId = dep.Parent

		// define the middle name of employee
		text, err := w.getData(cli, w.Gl.UrlFio, sotr.Name)
		if err != nil {
			w.Lg.Error("Get middle name:", "err", err.Error())
		} else {
			sotr.MidName = utils.ParseMidName(sotr, html.UnescapeString(text))
		}

		if sotr.MidName == "" {
			w.Lg.Warn("Middle name not found", "short_name", sotr.Name)
		}

		// define mobile phone
		if sotr.Mobile == "" {
			text, err = w.getData(cli, w.Gl.UrlMobile, sotr.Tabnum)
		}

		var mob *utils.Mobile

		if err != nil {
			w.Lg.Error("Mobile not found", "sotr_name", sotr.Name+sotr.MidName, "tabnum", sotr.Tabnum, "err", err)
		} else {
			mob, err = utils.ParseMobile(text)

			if err != nil {
				w.Lg.Error("Mobile parsing", "sotr_name", sotr.Name+sotr.MidName, "text", text, "err", err)
			} else if !mob.Success {
				w.Lg.Warn("Mobile get unsuccess", "sotr_name", sotr.Name+sotr.MidName, "tabnum", sotr.Tabnum, "responce", html.UnescapeString(text))
			}

			sotr.Mobile = mob.Data
		}

		item = sotr
	}

	_, err := w.Gl.Store.Save(context.TODO(), item)

	return err
}

func (w *Worker) getData(cli *req.Client, ajaxUrl, query string) (string, error) {
	var (
		errMsg ErrorMessage
		body   string // []byte
	)

	resp, err := cli.R().
		SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
		Get(ajaxUrl + url.PathEscape(query))
	if err != nil { // Error handling.
		err = fmt.Errorf("get mobile: error handling %w", err)

		w.Lg.Debug("Get Mobile error: raw content", "resp_dump", resp.Dump()) // Record raw content when error occurs.
	}

	if resp.IsErrorState() { // Status code >= 400.
		w.Lg.Error(errMsg.Message) // Record error message returned.
	}

	if resp.IsSuccessState() { // Status code is between 200 and 299.
		body = resp.String()
	}

	return (body), err
}

func (w *Worker) GetAvatar(_ context.Context, in <-chan Task, limit int32,
	depsCount *atomic.Int32, sotrCount *atomic.Int32, fileCollection map[string]AvatarInfo) {
	var errMsg ErrorMessage

	w.Lg.Info("Worker avatar: Start Getting...")

	var cli *req.Client

	defer func() {
		w.Lg.Info("Worker avatar: END")
	}()

	cli = w.Gl.ClientsPool

	cli.SetBaseURL(w.Gl.KbUrl).
		SetTimeout(w.Gl.HttpReqTimeout).
		SetLogger(&ReqLogger{Logger: *w.Lg})

	callback := func(info req.DownloadInfo) {
		if info.Response.Response != nil {
			fmt.Printf("downloaded %.2f%% (%s)\n", float64(info.DownloadedSize)/float64(info.Response.ContentLength)*100.0, info.Response.Request.URL.String())
		}
	}

	for task := range in {
		ava := task.Data

		cnt := depsCount.Load() + sotrCount.Load()
		if limit > 0 && cnt > limit {
			w.Lg.Info("Worker avatar: Count limited", "count", cnt)
			w.Gl.Done()

			return
		}

		w.Lg.Debug("Worker avatar:", "avatar", ava)

		// get head for compare file size
		r := cli.Head(ava).
			Do()

		if r.Err != nil {
			fmt.Println(r.Err.Error(), ava)
		}

		filename := filepath.Join(w.Gl.Avatars, ava)

		key := strings.Split(filepath.Base(ava), ".")[0]
		avaInfo, ok := fileCollection[key]

		if ok {
			if r.ContentLength == avaInfo.Size {
				w.Lg.Debug("Worker avatar: Skip existing >>>", "file", ava, "avaInfo", avaInfo)
				continue
			}
			// new name like "dir/8768768 (2).jpg"
			filename = filepath.Join(filepath.Dir(filename),
				fmt.Sprintf("%s (%d)%s",
					key,
					avaInfo.Num+1,
					filepath.Ext(avaInfo.ActualName)))
		}

		tFilename := filename + ".tmp"

		resp, err := cli.R().
			SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
			SetOutputFile(tFilename).
			SetDownloadCallback(callback).
			Get(ava)
		if err != nil { // Error handling.
			w.Lg.Error("Worker avatar: request handling", "error", err)

			err = os.Remove(tFilename)
			if err != nil {
				w.Lg.Error("Worker avatar: delete temp file", "error", err)
			}
		}

		if resp.IsErrorState() { // Status code >= 400.
			w.Lg.Error("Worker avatar:", "err", errMsg.Message) // Record error message returned.
		}

		if resp.IsSuccessState() { // Status code is between 200 and 299.
			w.Lg.Info("Worker avatar: downloaded", "avatar", ava, "syze", resp.ContentLength, "file_name", filename, "time", resp.TotalTime())

			err = os.Rename(tFilename, filename)
			if err != nil {
				w.Lg.Error("Worker avatar: rename temp file", "error", err)
			}
		}
	}
}
