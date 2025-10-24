package file

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
)

type FieldNameError struct {
	Name string
}

func (e FieldNameError) Error() string {
	return fmt.Sprintf("invalid field name \"%s\"", e.Name)
}

// string of directory path contains deps.json and sotr.json
type FileStore[T models.Item] struct {
	kbv1.UnimplementedStorServer

	BaseDir         string
	rwrDep, rwrSotr *bufio.ReadWriter
	flD, flS        *os.File
	mt              sync.Mutex
}

func NewFileStore[T models.Item](fname string) (*FileStore[T], error) {
	err := os.MkdirAll(fname, 0750)
	if err != nil {
		return nil, err
	}

	fPath := filepath.Join(string(fname), "dep.json")

	flD, err := os.OpenFile(fPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	fPath = filepath.Join(string(fname), "sotr.json")

	flS, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	return &FileStore[T]{
		BaseDir: fname,
		rwrDep:  bufio.NewReadWriter(bufio.NewReader(flD), bufio.NewWriter(flD)),
		rwrSotr: bufio.NewReadWriter(bufio.NewReader(flS), bufio.NewWriter(flS)),
		flD:     flD,
		flS:     flS,
	}, nil
}

func (f *FileStore[T]) Save(_ context.Context, item T) (_ *emptypb.Empty, err error) {
	b, err := json.Marshal(item)
	if err != nil {
		return
	}

	b = append(b, "\n"...)

	f.mt.Lock()
	defer f.mt.Unlock()

	if item.GetChildren() {
		_, err = f.rwrSotr.Write(b)
		// slog.Debug("Save to file sotr:", "item", item)
	} else {
		_, err = f.rwrDep.Write(b)
		// slog.Debug("Save to file dep:", "item", item)
	}

	return
}

func (f *FileStore[T]) Close() (err error) {
	e := f.rwrDep.Flush()
	if e != nil {
		err = fmt.Errorf("%w; %w", err, e)
	}

	e1 := f.rwrSotr.Flush()
	if e1 != nil {
		err = fmt.Errorf("%w; %w", err, e1)
	}

	e2 := f.flD.Close()
	if e2 != nil {
		err = fmt.Errorf("%w; %w", err, e2)
	}

	e3 := f.flS.Close()
	if e3 != nil {
		err = fmt.Errorf("%w; %w", err, e3)
	}

	return
}

func (f *FileStore[T]) GetDepsBy(ctx context.Context, query *kbv1.QueryDep) (deps []*kbv1.Dep, err error) {
	defer f.flS.Seek(0, io.SeekStart)

	var s, fieldValue string
	field := query.Field

	deps = make([]*kbv1.Dep, 0, 3)

	for err != io.EOF {
		dep := &kbv1.Dep{}

		s, err = f.rwrDep.ReadString('\n')

		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}

		err = protojson.Unmarshal([]byte(s), dep)
		if err != nil {
			slog.Error("GetDepsBy: unmurshall json", "error", err, "field", field, "json", s)
			continue
		}

		switch field {
		case kbv1.QueryDep_IDR:
			fieldValue = dep.Idr
		case kbv1.QueryDep_PARENT:
			fieldValue = dep.Parent
		default:
			// err = fmt.Errorf("invalid field name; field=%s", field)
			err = &FieldNameError{Name: "undefined"}
			return
		}

		if fieldValue == query.Str {
			deps = append(deps, dep)
		}
	}

	return
}

func (f *FileStore[T]) GetSotrsBy(ctx context.Context, query *kbv1.QuerySotr) (sotrs []*kbv1.Sotr, err error) {
	var (
		s          string
		fieldValue string
	)

	defer f.flS.Seek(0, io.SeekStart)

	field := query.Field

	sotrs = make([]*kbv1.Sotr, 0, 3)

	for {
		sotr := &kbv1.Sotr{}
		s, err = f.rwrSotr.ReadString('\n')

		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}

		err = protojson.Unmarshal([]byte(s), sotr)
		if err != nil {
			slog.Error("GetSotrsBy: unmurshall json", "error", err, "field", field, "json", s)
			continue
		}

		switch field {
		case kbv1.QuerySotr_MOBILE:
			fieldValue = sotr.Mobile[0]
		case kbv1.QuerySotr_FIO:
			fieldValue = sotr.Name
		case kbv1.QuerySotr_IDR:
			fieldValue = sotr.Idr
		case kbv1.QuerySotr_TABNUM:
			fieldValue = sotr.Tabnum
		default:
			// err = fmt.Errorf("invalid field name; field=%s", field)
			err = &FieldNameError{Name: "undefined"}
			return
		}

		if fieldValue == query.Str {
			sotrs = append(sotrs, sotr)
		}
	}

	return
}

func (f *FileStore[T]) GetDepByIdr1(ctx context.Context, idr *kbv1.QueryDep) (dep *kbv1.Dep, err error) {
	var s string
	sComp := fmt.Sprintf("{\"id\":\"%s\",", idr.Str)

	for err != io.EOF {
		s, err = f.rwrDep.ReadString('\n')
		if err != nil {
			return
		}
		if strings.HasPrefix(s, sComp) {
			err = json.Unmarshal([]byte(s), dep)
			if err != nil {
				return
			}
			break
		}
	}

	return
}

func (f *FileStore[T]) GetSotrByTabnum(ctx context.Context, tabnum *kbv1.QuerySotr) (sotr *kbv1.Sotr, err error) {
	var s string
	sComp := fmt.Sprintf("\"tabnum\":\"%s\",", tabnum.Str)
	sotr = &kbv1.Sotr{}

	for err != io.EOF {
		s, err = f.rwrSotr.ReadString('\n')

		if err != nil {
			return
		}

		if strings.Contains(s, sComp) {
			err = json.Unmarshal([]byte(s), sotr)

			if err != nil {
				return
			}
			break
		}
	}

	return
}

func (f *FileStore[T]) GetSotrByField(ctx context.Context, field string, quuery *kbv1.QuerySotr) (sotr *kbv1.Sotr, err error) {
	var (
		s          string
		fieldValue string
	)

	sotr = &kbv1.Sotr{}

	for err != io.EOF {
		s, err = f.rwrSotr.ReadString('\n')
		if err != nil {
			return
		}

		err = json.Unmarshal([]byte(s), sotr)
		if err != nil {
			slog.Error("GetSotrByField: unmurshall json", "error", err, "field", field, "json", s)
			continue
		}

		switch field {
		case "tabnum":
			fieldValue = sotr.Tabnum
		case "fio":
			fieldValue = sotr.Name
		default:
			err = &FieldNameError{Name: field}
			return
		}

		if fieldValue == quuery.Str {
			break
		}
	}

	return
}

// Migrate apply migrations to the DB
func (f *FileStore[T]) Migrate(ctx context.Context, down bool) error {
	return nil
}

// Retention deletes entries older than passed time
func (f *FileStore[T]) Retention(ctx context.Context, olderThan time.Time) error {
	return nil
}

func (f *FileStore[T]) PromCollector() prometheus.Collector {
	return nil
}
