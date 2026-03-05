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

// Task for getting response
type Task struct {
	Data string
	// Number of try to get response
	Num int
}

func NewTask(data string) *Task {
	return &Task{data, 0}
}

type Queue struct {
	list *list.List
	mu   sync.Mutex
}

func NewQueue() *Queue {
	return &Queue{
		list: list.New(),
	}
}
func (q *Queue) Push(data string) {
	q.withLock(func() {
		q.list.PushBack(data)
	})
}

func (q *Queue) Pop() string {
	var val string
	q.withLock(func() {
		if q.list.Len() == 0 {
			return
		}
		val = q.list.Remove(q.list.Front()).(string)
	})
	return val
}

func (q *Queue) Len() int {
	var length int
	q.withLock(func() {
		length = q.list.Len()
	})
	return length
}

func (q *Queue) withLock(fn func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	fn()
}

type ReqMessageError struct {
	Message string `json:"message"`
}

func (e *ReqMessageError) Error() string {
	return e.Message
}

func (e *ReqMessageError) Is(err error) bool {
	_, ok := err.(*ReqMessageError)
	return ok
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
	// mu          sync.Mutex
	Name        string
	QueueDep    *Queue
	QueueAvatar *Queue
	Conf        *Config
	// IsData       chan struct{}
	// IsDataA      chan struct{}
	Lg           *slog.Logger
	httpClient   *req.Client
	PollInterval *time.Duration
}

func NewWorker(conf *Config, name string, debugLevel int, logger *slog.Logger) *Worker {
	lg := logger.With("worker", name)
	// isData := make(chan struct{}, 1500)
	// IsDataA := make(chan struct{}, 1500)
	cli := httpclient.NewHTTPClient(debugLevel, conf.Headers)
	cli.SetBaseURL(conf.KbUrl).
		SetTimeout(conf.HttpReqTimeout).
		SetLogger(&ReqLogger{Logger: *lg})

	return &Worker{
		Name:        name,
		QueueDep:    NewQueue(),
		QueueAvatar: NewQueue(),
		Conf:        conf,
		Lg:          lg,
		// IsData:       isData,
		// IsDataA:      IsDataA,
		httpClient:   cli,
		PollInterval: &conf.DispPollInterval,
	}
}

// GetRazd ...
func (w *Worker) GetRazd(ctx context.Context, in chan Task, out chan models.Item, limit int32, depsCount *atomic.Int32, sotrsCount *atomic.Int32) (err error) {
	var (
		errMsg *ReqMessageError
		raw    []json.RawMessage
	)

	w.Lg.Debug("Worker: Start Getting...")
	defer w.Lg.Debug("Worker: END...")
	// defer close(w.IsDataA)
	// defer close(w.IsData)

	cli := w.httpClient

	for {
		select {

		case task, ok := <-in:
			if !ok {
				return
			}
			cnt := depsCount.Load() + sotrsCount.Load()

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
			resp, e := cli.R().
				SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
				EnableDump().            // Enable dump at request level, only print dump content if there is an error or some unknown situation occurs to help troubleshoot.
				// SetSuccessResult(&DepsResponse). // Unmarshal response body into userInfo automatically if status code is between 200 and 299.
				SetContext(ctx).
				Get(w.Conf.UrlRazd + task.Data)

			select {
			case <-ctx.Done():
				w.Lg.Info("Worker: cancel done", "err", ctx.Err().Error())
				return ctx.Err()
			default:
				if e != nil {
					w.Lg.Debug("Worker: error handling", "resp dump", resp.Dump()) // Record raw content when error occurs.
					err = e
					return
				}
			}

			if resp.IsErrorState() { // Status code >= 400.
				w.Lg.Error("Worker:", "err", errMsg.Message) // Record error message returned.
				err = errMsg
				return
			}

			if resp.IsSuccessState() { // Status code is between 200 and 299.

				body, e := resp.ToBytes()
				if e != nil {
					w.Lg.Error("Worker: get body:", "err", e, "delay", resp.TotalTime())
				}

				if e := json.Unmarshal(body, &raw); e != nil {
					w.Lg.Error("Worker: unmurshal body to []Raw:", "err", e, "delay", resp.TotalTime())
				}

				DepsResponse := make([]*kbv1.Dep, len(raw))
				// opts := &protojson.UnmarshalOptions{DiscardUnknown: true}

				for i, rm := range raw {
					dep := &models.Dep{} // Новое сообщение
					if e := json.Unmarshal([]byte(rm), dep); e != nil {
						w.Lg.Error("Worker: unmurshal []Raw :", "err", e, "delay", resp.TotalTime())
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

					// requeue with backoff in a goroutine to avoid blocking the caller
					go func(data string, num int) {
						backoff := time.Duration(1<<num) * time.Second
						if backoff > 10*time.Second {
							backoff = 10 * time.Second
						}
						select {
						case <-time.After(backoff):
						case <-ctx.Done():
							return
						}
						select {
						case in <- Task{data, num}:
						case <-ctx.Done():
						}
					}(task.Data, task.Num+1)
					continue
				}

				for _, d := range DepsResponse {
					if d.GetChildren() {
						w.QueueDep.Push(d.Idr)
						depsCount.Add(1)
						// select {
						// case w.IsData <- struct{}{}:
						// default:
						// }
					} else {
						sotrsCount.Add(1)
					}
					// e := w.PrepareItem(ctx, d)
					// if e != nil {
					// 	w.Lg.Error("Worker: Prepare dep", "err", e, "dep", d)
					// }
					select {
					case out <- w.PrepareItem(ctx, d):
					case <-ctx.Done():
					}
				}
			}

			lenQ := w.QueueDep.Len()
			w.Lg.Debug("Worker Len of QueueDep:", "len", lenQ)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return
}

// Dispatcher wolking accross worker queues for forwarding idr/avatar to worker input chanal
func (w *Worker) Dispatcher(ctx context.Context, dispName string, queue *Queue, out chan<- Task) { // , isData <-chan struct{}
	var (
		count, lenQ int
		s, message  string
		timeStart   time.Time
	)

	w.Lg.Debug(fmt.Sprintf("Dispatcher %s: Getting data from Queue...", dispName))
	defer func() {
		w.Lg.Debug(fmt.Sprintf("Dispatcher %s: END %s", dispName, message))
	}()

	ticker := time.NewTicker(*w.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for queue.Len() > 0 {
				timeStart = time.Now()
				// get data from queue
				s = queue.Pop()

				// send data to out chanal (razdCh)
				select {
				case out <- *NewTask(s):
					count++

				case <-ctx.Done():
					// return task to queue
					queue.Push(s)
					// TODO save queue for continue in the next run
					lenQ = queue.Len()
					w.Lg.Info(fmt.Sprintf("Dispatcher %s: general cancel:", dispName), "err", ctx.Err().Error(), "QueueLen", lenQ) // , "DepsResponseQueueLen", lenQD)
					return
				}
				lenQ = queue.Len()
				w.Lg.Info(fmt.Sprintf("Dispatcher %s: Get from queue", dispName), "count", count, "data", s, "QueueLen", lenQ, "delay", time.Since(timeStart))
			}

		case <-ctx.Done():
			message = "with general cancel in loop"

			return
		}
	}
}

// PrepareItem define the item as a DepsResponse or an employee and save one
func (w *Worker) PrepareItem(ctx context.Context, item models.Item) models.Item {
	ATTEMPT := 1 // number of attempts for get data if success field = false
	dep := item.(*kbv1.Dep)
	dep.Text = html.UnescapeString(dep.Text)

	if !item.GetChildren() {
		sotr := utils.ParseSotr(dep.Text)

		// send url Avatar image to queue for download
		w.QueueAvatar.Push(sotr.Avatar)

		// select {
		// case w.IsDataA <- struct{}{}:
		// default:
		// }
		sotr.Children = dep.Children
		sotr.Idr = dep.Idr
		sotr.ParentId = dep.Parent

		// define the middle name of employee
		text, err := w.getData(ctx, w.Conf.UrlFio, sotr.Name)
		if err != nil {
			w.Lg.Error("Get middle name:", "err", err.Error())
		} else {
			sotr.MidName = utils.ParseMidName(sotr, html.UnescapeString(text))
		}

		if sotr.MidName == "" {
			w.Lg.Warn("Middle name not found", "short_name", sotr.Name)
		}

		sotrFullName := fmt.Sprintf("%s %s", sotr.Name, sotr.MidName)

		// define mobile phone
		text = ""
		text, err = w.getData(ctx, w.Conf.UrlSotr, sotr.Tabnum)

		if err != nil || !utils.HasValidMobile(text) {
			w.Lg.Error("Mobile data not found", "sotr_name", sotrFullName, "tabnum", sotr.Tabnum, "err", err, "text", text)
		} else {

			var mob *utils.Mobile

			for i := 0; i < ATTEMPT; i++ {
				// if len(sotr.Mobile) == 0 {
				// 	break
				// }
				text, err = w.getData(ctx, w.Conf.UrlMobile, sotr.Tabnum)

				if err != nil {
					w.Lg.Error("Mobile not found", "sotr_name", sotrFullName, "tabnum", sotr.Tabnum, "err", err)
					break
				}

				mob, err = utils.ParseMobile(text)

				if err != nil {
					w.Lg.Error("Mobile parsing", "sotr_name", sotrFullName, "text", text, "err", err)
					break
				}

				if !mob.Success {
					w.Lg.Warn(fmt.Sprintf("#%d: Mobile get unsuccess", i+1), "sotr_name", sotrFullName, "tabnum", sotr.Tabnum, "responce", slog.String("message", text)) //html.UnescapeString(text))
					time.Sleep(time.Duration(1<<(7+i)) * time.Millisecond)
					continue
				}

				if mob.Data != "" {
					sotr.Mobile = strings.Split(mob.Data, ",")
					w.Lg.Debug(fmt.Sprintf("#%d: Mobile get success", i+1), "sotr_name", sotrFullName, "tabnum", sotr.Tabnum, "responce", slog.String("message", text), "mob.Data", mob.Data) //html.UnescapeString(text))
				}
				break

			}
		}
		item = sotr
	}
	// _, err := w.Conf.Store.Save(ctx, item)

	return item
}

func (w *Worker) getData(_ context.Context, ajaxUrl, query string) (string, error) {
	var (
		errMsg ReqMessageError
		body   string // []byte
	)

	qURL := fmt.Sprintf("%s%s", ajaxUrl, url.PathEscape(query))

	cli := w.httpClient
	resp, err := cli.R().
		SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
		//	SetContext(ctx).
		Get(qURL)

	if err != nil { // Error handling.
		w.Lg.Debug("Get Data: raw content", "url", qURL, "resp_dump", resp.Dump()) // Record raw content when error occurs.
		err = fmt.Errorf("get url %s: error handling %w", qURL, err)
	}

	if resp.IsErrorState() { // Status code >= 400.
		w.Lg.Error(errMsg.Message) // Record error message returned.
	}

	if resp.IsSuccessState() { // Status code is between 200 and 299.
		body = resp.String()
		// fmt.Printf("BODY:\n%s\nDUMP:%s\n", body, resp.Dump())
	}

	return (body), err
}

func (w *Worker) GetAvatar(ctx context.Context, in <-chan Task, limit int32,
	depsCount *atomic.Int32, sotrsCount *atomic.Int32, fileCollection map[string]AvatarInfo) error {
	var errMsg ReqMessageError

	w.Lg.Debug("Worker avatar: Start Getting...")

	defer func() {
		w.Lg.Debug("Worker avatar: END...")
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

			cnt := depsCount.Load() + sotrsCount.Load()
			if limit > 0 && cnt > limit {
				w.Lg.Info("Worker avatar: Count limited", "count", cnt)
				return &TaskLimitExceededError{val: int(cnt)}
			}

			w.Lg.Debug("Worker avatar:", "avatar", ava)

			// get head for compare file size
			r, e := cli.R().
				// SetContext(ctx).
				Head(ava)

			if e != nil {
				fmt.Println(e.Error(), ava)
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
	return nil
}
