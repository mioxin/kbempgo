package worker

import (
	"container/list"
	"context"
	"encoding/json"
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
	httpclient "github.com/mioxin/kbempgo/internal/http_client"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/mioxin/kbempgo/internal/utils"
	"google.golang.org/protobuf/proto"
)

type Task struct {
	Data string
	Num  int
}

func NewTask(data string) *Task {
	return &Task{data, 0}
}

type ReqMessageError struct {
	Message string `json:"message"`
}

type TaskLimitExceededError struct {
	val int
}

func (e *TaskLimitExceededError) Error() string {
	return fmt.Sprintf("limit of requests is exeded, count: %d", e.val)
}

func (e *TaskLimitExceededError) Is(err error) bool {
	_, ok := err.(*TaskLimitExceededError)
	return ok
}

// TryLimit is retry get razd if empty one
const TryLimit int = 3

type Item interface {
	proto.Message
	GetChildren() bool
}

type Worker struct {
	mu          sync.Mutex
	Name        string
	QueueDep    *list.List
	QueueAvatar *list.List
	Conf        *Config
	IsData      chan struct{}
	IsDataA     chan struct{}
	Lg          *slog.Logger
	httpClient  *req.Client
}

func NewWorker(conf *Config, name string, debugLevel int, logger *slog.Logger) *Worker {
	lg := logger.With("worker", name)
	isData := make(chan struct{}, 1500)
	IsDataA := make(chan struct{}, 1500)
	cli := httpclient.NewHTTPClient(debugLevel, conf.Headers)
	cli.SetBaseURL(conf.KbUrl).
		SetTimeout(conf.HttpReqTimeout).
		SetLogger(&ReqLogger{Logger: *lg})

	return &Worker{Name: name,
		QueueDep:    list.New(),
		QueueAvatar: list.New(),
		Conf:        conf,
		Lg:          lg,
		IsData:      isData,
		IsDataA:     IsDataA,
		httpClient:  cli,
	}
}

// GetRazd ...
func (w *Worker) GetRazd(ctx context.Context, in chan Task, limit int32, DepsResponseCount *atomic.Int32, sotrCount *atomic.Int32) (err error) {
	var (
		errMsg ReqMessageError
		// cli *req.Client
	)

	w.Lg.Info("Worker: Start Getting...")

	defer func() {
		close(w.IsDataA)
		close(w.IsData)
		w.Lg.Info("Worker: END")
	}()

	cli := w.httpClient

	for {
		select {

		case task := <-in:
			cnt := DepsResponseCount.Load() + sotrCount.Load()

			if limit > 0 && cnt > limit {
				w.Lg.Info("Worker: Count limited", "count", cnt)
				return &TaskLimitExceededError{val: int(cnt)}
			}

			if task.Num > TryLimit {
				w.Lg.Warn("Worker: Out of retry limit", "try", task.Num, "req_dep", task.Data)
				break
			}

			w.Lg.Debug("Worker:", "dep", task.Data, "try", task.Num)

			// DepsResponse := make([]*kbv1.Dep, 0)

			// retry if successful req but empty DepsResponse
			resp, err := cli.R().
				SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
				EnableDump().            // Enable dump at request level, only print dump content if there is an error or some unknown situation occurs to help troubleshoot.
				// SetSuccessResult(&DepsResponse). // Unmarshal response body into userInfo automatically if status code is between 200 and 299.
				// SetContext(ctx).
				Get(w.Conf.UrlRazd + task.Data)

			select {
			case <-ctx.Done():
				w.Lg.Info("Worker:", "err", ctx.Err().Error())
				return ctx.Err()
			default:
				if err := ctx.Err(); err != nil {
					return err
				}
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

				body, err := resp.ToBytes()
				if err != nil {
					w.Lg.Error("Worker: get body:", "err", err, "delay", resp.TotalTime())
				}

				var raw []json.RawMessage
				if err := json.Unmarshal(body, &raw); err != nil {
					w.Lg.Error("Worker: unmurshal body to []Raw:", "err", err, "delay", resp.TotalTime())
				}

				DepsResponse := make([]*kbv1.Dep, len(raw))
				// opts := &protojson.UnmarshalOptions{DiscardUnknown: true}

				for i, rm := range raw {
					dep := &models.Dep{} // Новое сообщение

					if err := json.Unmarshal([]byte(rm), dep); err != nil {
						w.Lg.Error("Worker: unmurshal []Raw :", "err", err, "delay", resp.TotalTime())
					}

					DepsResponse[i] = dep.Conv2Kbv().GetDep()
				}

				// construct string for debug output
				rBytes := []byte{}
				for _, dep := range DepsResponse {
					rBytes = append(rBytes, []byte(dep.Idr)...)
					rBytes = append(rBytes, []byte("; ")...)
				}

				w.Lg.Debug("Worker: responce:", "razd", string(rBytes), "DepsResponse_length", len(DepsResponse), "delay", resp.TotalTime())

				// retry get razd
				if len(DepsResponse) == 0 { // && resp.TotalTime() > 4*time.Second {
					w.Lg.Warn("Worker: Empty response ", "try", task.Num, "req_dep", task.Data, "resp", resp.Dump(), "delay", resp.TotalTime())

					in <- Task{task.Data, task.Num + 1}
					continue
				}

				for _, d := range DepsResponse {
					if d.GetChildren() {
						w.mu.Lock()
						w.QueueDep.PushBack(d.Idr)
						w.mu.Unlock()

						DepsResponseCount.Add(1)

						select {
						case w.IsData <- struct{}{}:
						default:
						}
					} else {
						sotrCount.Add(1)
					}

					err := w.PrepareItem(ctx, d)
					if err != nil {
						w.Lg.Error("Worker: Prepare dep", "err", err, "dep", d)
					}
				}
			}

			w.mu.Lock()
			lenQ := w.QueueDep.Len()
			w.mu.Unlock()

			w.Lg.Debug("Worker Len of QueueDep:", "len", lenQ)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return
}

// Dispatcher wolking accross worker queues for forwarding idr/avatar to worker input chanal
func (w *Worker) Dispatcher(ctx context.Context, dispName string, queue *list.List, out chan<- Task, isData <-chan struct{}) {
	var (
		count, lenQ int
		s           string
	)

	defer w.Lg.Info(fmt.Sprintf("Dispatcher %s: END...", dispName))

	send2chan := func(q *list.List, outCh chan<- Task) (ok bool) {
		ok = true

		w.mu.Lock()

		lenQ = q.Len()

		w.mu.Unlock()

		if lenQ == 0 {
			w.Lg.Info(fmt.Sprintf("Dispatcher %s: isData when Queue len =0", dispName))
			return
		}

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

			w.Lg.Info(fmt.Sprintf("Dispatcher %s: general cancel:", dispName), "err", ctx.Err().Error(), "QueueLen", lenQ) // , "DepsResponseQueueLen", lenQD)
			ok = false

			return
		}

		return
	}

	for {
		// if the dispatcher faster a workers then isData indicate new data in the queue
		// if w.QueueDep.Len() == 0 {
		w.Lg.Debug(fmt.Sprintf("Dispatcher %s: Getting data from Queue...", dispName))

		timeStart := time.Now()

		select {
		case _, ok := <-isData:
			if err := ctx.Err(); err != nil {
				return
			}

			if !ok {
				return
			}

			if !send2chan(queue, out) {
				return
			}

			w.mu.Lock()
			lenQ := queue.Len()
			w.mu.Unlock()

			w.Lg.Info(fmt.Sprintf("Dispatcher %s: Get from queue", dispName), "count", count, "data", s, "QueueLen", lenQ, "delay", time.Since(timeStart))

		case <-ctx.Done():
			w.mu.Lock()
			lenQ := queue.Len()
			w.mu.Unlock()

			w.Lg.Debug(fmt.Sprintf("Dispatcher %s: general cancel in loop:", dispName), "err", ctx.Err().Error(), "QueueLen", lenQ, "IsData", len(isData)) // , "DepsResponseQueueLen", lenQD, "DepsResponseIsData", len(w.isData))

			return
		}
		// }
	}
}

// PrepareItem define the item as a DepsResponse or an employee and save one
func (w *Worker) PrepareItem(ctx context.Context, item models.Item) error {
	dep := item.(*kbv1.Dep)
	dep.Text = html.UnescapeString(dep.Text)

	if !item.GetChildren() {
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
		text, err := w.getData(w.Conf.UrlFio, sotr.Name)
		if err != nil {
			w.Lg.Error("Get middle name:", "err", err.Error())
		} else {
			sotr.MidName = utils.ParseMidName(sotr, html.UnescapeString(text))
		}

		if sotr.MidName == "" {
			w.Lg.Warn("Middle name not found", "short_name", sotr.Name)
		}

		// define mobile phone
		if len(sotr.Mobile) == 0 {
			text, err = w.getData(w.Conf.UrlMobile, sotr.Tabnum)
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
			if mob.Data != "" {
				sotr.Mobile = strings.Split(mob.Data, ",")
			}

		}

		item = sotr
	}

	_, err := w.Conf.Store.Save(ctx, item)

	return err
}

func (w *Worker) getData(ajaxUrl, query string) (string, error) {
	var (
		errMsg ReqMessageError
		body   string // []byte
	)

	cli := w.httpClient
	resp, err := cli.R().
		SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
		Get(ajaxUrl + url.PathEscape(query))
	if err != nil { // Error handling.
		w.Lg.Debug("Get Data: raw content", "url", ajaxUrl, "resp_dump", resp.Dump()) // Record raw content when error occurs.
		err = fmt.Errorf("get url %s: error handling %w", ajaxUrl, err)
	}

	if resp.IsErrorState() { // Status code >= 400.
		w.Lg.Error(errMsg.Message) // Record error message returned.
	}

	if resp.IsSuccessState() { // Status code is between 200 and 299.
		body = resp.String()
	}

	return (body), err
}

func (w *Worker) GetAvatar(ctx context.Context, in <-chan Task, limit int32,
	DepsResponseCount *atomic.Int32, sotrCount *atomic.Int32, fileCollection map[string]AvatarInfo) (err error) {
	var errMsg ReqMessageError

	w.Lg.Info("Worker avatar: Start Getting...")

	defer func() {
		w.Lg.Info("Worker avatar: END")
	}()

	cli := w.httpClient

	callback := func(info req.DownloadInfo) {
		if info.Response.Response != nil {
			fmt.Printf("downloaded %.2f%% (%s)\n", float64(info.DownloadedSize)/float64(info.Response.ContentLength)*100.0, info.Response.Request.URL.String())
		}
	}

	for {
		select {

		case task := <-in:
			ava := task.Data

			cnt := DepsResponseCount.Load() + sotrCount.Load()
			if limit > 0 && cnt > limit {
				w.Lg.Info("Worker avatar: Count limited", "count", cnt)
				return &TaskLimitExceededError{val: int(cnt)}
			}

			w.Lg.Debug("Worker avatar:", "avatar", ava)

			// get head for compare file size
			r := cli.Head(ava).
				Do()

			if r.Err != nil {
				fmt.Println(r.Err.Error(), ava)
			}

			filename := filepath.Join(w.Conf.Avatars, ava)

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
				w.Lg.Info("Worker avatar: downloaded", "avatar", ava, "syze", resp.ContentLength, "file_name", filename, "delay", resp.TotalTime())

				err = os.Rename(tFilename, filename)
				if err != nil {
					w.Lg.Error("Worker avatar: rename temp file", "error", err)
				}
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return
}
