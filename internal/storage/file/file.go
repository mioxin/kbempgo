package file

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FieldNameError struct {
	Name string
}

func (e FieldNameError) Error() string {
	return fmt.Sprintf("invalid field name \"%s\"", e.Name)
}

// string of directory path contains DepsResponse.json and SotrsResponse.json
type FileStore struct {
	kbv1.UnimplementedStorAPIServer

	BaseDir         string
	rwrDep, rwrSotr *bufio.ReadWriter
	flD, flS        *os.File
	mt              sync.Mutex
	Log             *slog.Logger
}

func NewFileStore(fname string, log *slog.Logger) (*FileStore, error) {
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

	flS, err := os.OpenFile(fPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	return &FileStore{
		BaseDir: fname,
		rwrDep:  bufio.NewReadWriter(bufio.NewReader(flD), bufio.NewWriter(flD)),
		rwrSotr: bufio.NewReadWriter(bufio.NewReader(flS), bufio.NewWriter(flS)),
		flD:     flD,
		flS:     flS,
		Log:     log.With("storage", "files"),
	}, nil
}

func (f *FileStore) Save(_ context.Context, item models.Item) (_ *emptypb.Empty, err error) {

	if !item.GetChildren() {
		if sotr, ok := any(item).(*kbv1.Sotr); ok {
			err = f.saveSotr(sotr)

			if err != nil {
				return
			}

			return
		}

		err = fmt.Errorf("expected *kbv1.Sotr, got %T", item)

	} else if dep, ok := any(item).(*kbv1.Dep); ok {
		err = f.saveDep(dep)

		if err != nil {
			return
		}
		return

	} else {
		err = fmt.Errorf("expected *kbv1.Dep, got %T", item)
	}

	return
}

func (f *FileStore) saveDep(dep *kbv1.Dep) (err error) {
	var DepsResponse []*kbv1.Dep
	var b []byte

	// get dep if exists for define double raw
	DepsResponse, err = f.GetDepsBy(context.Background(), &kbv1.DepRequest{Field: kbv1.DepRequest_IDR, Str: dep.Idr})
	if err != nil {
		return
	}

	// check double raw
	for _, d := range DepsResponse {
		if d.Parent == dep.Parent && d.Text == dep.Text {
			return
		}
	}

	marshaler := protojson.MarshalOptions{
		EmitUnpopulated: true, // for sure includes bool fields =  false/0/""
	}
	b, err = marshaler.Marshal(dep)
	if err != nil {
		return
	}

	b = append(b, "\n"...)

	f.mt.Lock()
	defer f.mt.Unlock()

	_, err = f.rwrDep.Write(b)
	if err != nil {
		return
	}

	f.Log.Debug("saved", "dep", string(b))
	return
}

func (f *FileStore) saveSotr(sotr *kbv1.Sotr) (err error) {

	ctx := context.Background()
	var SotrsResponse []*kbv1.Sotr

	// get SotrsResponse if exists for define double raw
	SotrsResponse, err = f.GetSotrsBy(ctx, &kbv1.SotrRequest{Field: kbv1.SotrRequest_TABNUM, Str: sotr.Tabnum})
	if err != nil {
		return
	}

	// doublicates found
	if len(SotrsResponse) > 0 {

		f.Log.Debug("saved: doublicates by Tabnum is found", "num", len(SotrsResponse))

		sort.Slice(SotrsResponse, func(i, j int) bool {
			if SotrsResponse[i].Date == nil {
				return true
			}
			if SotrsResponse[j].Date == nil {
				return false
			}
			return SotrsResponse[i].Date.AsTime().Before(SotrsResponse[j].Date.AsTime())
		})
		// get newest raw
		oldSotr := SotrsResponse[len(SotrsResponse)-1]

		// if double raw exists then compare for define difference
		diff, _ := kbv1.CompareSotr(oldSotr, sotr)
		// if diff exists than save diff to history and update old SotrsResponse
		if len(diff) > 0 {
			f.Log.Debug("saved: doublicate have diffs", "num", len(diff))

			hs := make([]*kbv1.History, 0)
			for _, df := range diff {

				value := ""

				switch t := df.Val.(type) {
				case string:
					value = string(t)
				case []string:
					value = strings.Join(t, ",")
				}

				hs = append(hs, &kbv1.History{
					Date:     timestamppb.Now(),
					Field:    df.FieldName,
					OldValue: value,
					SotrId:   oldSotr.Id,
				})
			}

			f.Log.Debug("saved: history of diff", "hs", hs)
			_, err = f.Update(ctx, &kbv1.UpdateSotrRequest{Sotr: sotr, HistoryList: hs})

			if err != nil {
				f.Log.Error("saved: update", "err", err)
			}
		}
		return
	}

	marshaler := protojson.MarshalOptions{
		EmitUnpopulated: true, // for sure includes bool fields =  false/0/""
	}

	b, err := marshaler.Marshal(sotr)
	if err != nil {
		return
	}

	b = append(b, "\n"...)

	f.mt.Lock()
	defer f.mt.Unlock()

	_, err = f.rwrSotr.Write(b)
	if err != nil {
		err = fmt.Errorf("error save Sotr to Stor: %w", err)
		return
	}

	f.Log.Debug("saved", "sotr", string(b))

	return
}

func (f *FileStore) Update(_ context.Context, query *kbv1.UpdateSotrRequest) (_ *emptypb.Empty, err error) {
	marshaler := protojson.MarshalOptions{
		EmitUnpopulated: true, // for sure includes bool fields =  false/0/""
	}

	b, err := marshaler.Marshal(query.Sotr)
	if err != nil {
		return
	}
	b = append(b, "\n"...)

	f.mt.Lock()
	defer f.mt.Unlock()

	_, err = f.rwrSotr.Write(b)

	return
}

func (f *FileStore) Flush(ctx context.Context, _ *emptypb.Empty) (_ *emptypb.Empty, err error) {
	f.mt.Lock()
	defer f.mt.Unlock()

	errs := make([]error, 0)

	e := f.rwrDep.Flush()
	if e != nil {
		err = fmt.Errorf("%w; %w", err, e)
		errs = append(errs, err)
	}

	e1 := f.rwrSotr.Flush()
	if e1 != nil {
		err = fmt.Errorf("%w; %w", err, e1)
		errs = append(errs, err)
	}

	err = errors.Join(errs...)
	return
}

func (f *FileStore) Close() (err error) {
	f.mt.Lock()
	defer f.mt.Unlock()

	errs := make([]error, 0)

	e := f.rwrDep.Flush()
	if e != nil {
		err = fmt.Errorf("%w; %w", err, e)
		errs = append(errs, err)
	}

	e1 := f.rwrSotr.Flush()
	if e1 != nil {
		err = fmt.Errorf("%w; %w", err, e1)
		errs = append(errs, err)
	}

	e2 := f.flD.Close()
	if e2 != nil {
		err = fmt.Errorf("%w; %w", err, e2)
		errs = append(errs, err)
	}

	e3 := f.flS.Close()
	if e3 != nil {
		err = fmt.Errorf("%w; %w", err, e3)
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (f *FileStore) GetDepsBy(ctx context.Context, query *kbv1.DepRequest) (DepsResponse []*kbv1.Dep, err error) {
	f.mt.Lock()
	defer f.mt.Unlock()
	f.flD.Seek(0, io.SeekStart)

	var s string
	field := query.Field

	DepsResponse = make([]*kbv1.Dep, 0, 3)
	findByFieldVal := func(checkEqualValue func(d *kbv1.Dep, val string) bool) (err error) {

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
				f.Log.Error("GetDepsBy: unmurshall json", "error", err, "field", field, "json", s)
				continue
			}

			if checkEqualValue(dep, query.Str) {
				DepsResponse = append(DepsResponse, dep)
			}
		}
		return
	}

	switch field {
	case kbv1.DepRequest_NONE:
		err = findByFieldVal(func(d *kbv1.Dep, val string) bool {
			return true
		})

	case kbv1.DepRequest_IDR:
		err = findByFieldVal(func(d *kbv1.Dep, val string) bool {
			return d.Idr == val
		})

	case kbv1.DepRequest_PARENT:
		err = findByFieldVal(func(d *kbv1.Dep, val string) bool {
			return d.Parent == val
		})

	default:
		// err = fmt.Errorf("invalid field name; field=%s", field)
		err = &FieldNameError{Name: "undefined"}
		return
	}

	return
}

func (f *FileStore) GetSotrsBy(ctx context.Context, query *kbv1.SotrRequest) (SotrsResponse []*kbv1.Sotr, err error) {
	var s string

	f.mt.Lock()
	defer f.mt.Unlock()

	f.flS.Seek(0, io.SeekStart)
	field := query.Field

	SotrsResponse = make([]*kbv1.Sotr, 0, 3)
	findByFieldVal := func(checkEqualValue func(d *kbv1.Sotr, val string) bool) (err error) {

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

			// opts := &protojson.UnmarshalOptions{DiscardUnknown: true, AllowPartial: true}
			err = protojson.Unmarshal([]byte(s), sotr)
			if err != nil {
				f.Log.Error("GetSotrsBy: unmurshall json", "error", err, "field", field, "json", s)
				continue
			}
			if checkEqualValue(sotr, query.Str) {
				SotrsResponse = append(SotrsResponse, sotr)
			}
		}

		return
	}

	switch field {
	case kbv1.SotrRequest_NONE:
		err = findByFieldVal(func(d *kbv1.Sotr, val string) bool {
			return true
		})

	case kbv1.SotrRequest_IDR:
		err = findByFieldVal(func(d *kbv1.Sotr, val string) bool {
			return d.Idr == val
		})
	case kbv1.SotrRequest_MOBILE:
		err = findByFieldVal(func(d *kbv1.Sotr, val string) bool {
			return d.Mobile[0] == val
		})
	case kbv1.SotrRequest_FIO:
		err = findByFieldVal(func(d *kbv1.Sotr, val string) bool {
			return d.Name == val
		})
	case kbv1.SotrRequest_TABNUM:
		err = findByFieldVal(func(d *kbv1.Sotr, val string) bool {
			return d.Tabnum == val
		})
	default:
		// err = fmt.Errorf("invalid field name; field=%s", field)
		err = &FieldNameError{Name: "undefined"}
		return
	}

	return
}

func (f *FileStore) PromCollector() prometheus.Collector {
	return nil
}
