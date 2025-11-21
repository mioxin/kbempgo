package file

import (
	"bufio"
	"context"
	"encoding/json"
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

// string of directory path contains deps.json and sotrs.json
type FileStore[T models.Item] struct {
	kbv1.UnimplementedStorServer

	BaseDir         string
	rwrDep, rwrSotr *bufio.ReadWriter
	flD, flS        *os.File
	mt              sync.Mutex
	Log             *slog.Logger
}

func NewFileStore[T models.Item](fname string, log *slog.Logger) (*FileStore[T], error) {
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

	return &FileStore[T]{
		BaseDir: fname,
		rwrDep:  bufio.NewReadWriter(bufio.NewReader(flD), bufio.NewWriter(flD)),
		rwrSotr: bufio.NewReadWriter(bufio.NewReader(flS), bufio.NewWriter(flS)),
		flD:     flD,
		flS:     flS,
		Log:     log.With("storage", "files"),
	}, nil
}

func (f *FileStore[T]) Save(_ context.Context, item T) (_ *emptypb.Empty, err error) {

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

func (f *FileStore[T]) saveDep(dep *kbv1.Dep) (err error) {
	var deps []*kbv1.Dep

	// get dep if exists for define double raw
	deps, err = f.GetDepsBy(context.Background(), &kbv1.QueryDep{Field: kbv1.QueryDep_IDR, Str: dep.Idr})
	if err != nil {
		return
	}

	// check double raw
	for _, d := range deps {
		if d.Parent == dep.Parent && d.Text == dep.Text {
			return
		}
	}

	marshaler := protojson.MarshalOptions{
		EmitUnpopulated: true, // for sure includes bool fields =  false/0/""
	}
	b, err := marshaler.Marshal(dep)
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

func (f *FileStore[T]) saveSotr(sotr *kbv1.Sotr) (err error) {

	ctx := context.Background()
	var sotrs []*kbv1.Sotr

	// get sotrs if exists for define double raw
	sotrs, err = f.GetSotrsBy(ctx, &kbv1.QuerySotr{Field: kbv1.QuerySotr_TABNUM, Str: sotr.Tabnum})
	if err != nil {
		return
	}

	// doublicates found
	if len(sotrs) > 0 {

		f.Log.Debug("saved: doublicates by Tabnum is found", "num", len(sotrs))

		sort.Slice(sotrs, func(i, j int) bool {
			if sotrs[i].Date == nil {
				return true
			}
			if sotrs[j].Date == nil {
				return false
			}
			return sotrs[i].Date.AsTime().Before(sotrs[j].Date.AsTime())
		})
		// get newest raw
		oldSotr := sotrs[len(sotrs)-1]

		// if double raw exists then compare for define difference
		diff, _ := kbv1.CompareSotr(oldSotr, sotr)
		// if diff exists than save diff to history and update old sotrs
		if len(diff) > 0 {
			f.Log.Debug("saved: doublicate have diffs", "nim", len(diff))

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
			_, err = f.Update(ctx, &kbv1.QueryUpdateSotr{Sotr: sotr, HistoryList: hs})

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

func (f *FileStore[T]) Update(_ context.Context, query *kbv1.QueryUpdateSotr) (_ *emptypb.Empty, err error) {
	b, err := json.Marshal(query.Sotr)
	if err != nil {
		return
	}
	b = append(b, "\n"...)

	f.mt.Lock()
	defer f.mt.Unlock()

	_, err = f.rwrSotr.Write(b)

	return
}

func (f *FileStore[T]) Flush() (err error) {
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

	return errors.Join(errs...)
}

func (f *FileStore[T]) Close() (err error) {
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

func (f *FileStore[T]) GetDepsBy(ctx context.Context, query *kbv1.QueryDep) (deps []*kbv1.Dep, err error) {
	f.mt.Lock()
	defer f.mt.Unlock()
	f.flD.Seek(0, io.SeekStart)

	var s string
	field := query.Field

	deps = make([]*kbv1.Dep, 0, 3)
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
				deps = append(deps, dep)
			}
		}
		return
	}

	switch field {
	case kbv1.QueryDep_EMPTY:
		err = findByFieldVal(func(d *kbv1.Dep, val string) bool {
			return true
		})

	case kbv1.QueryDep_IDR:
		err = findByFieldVal(func(d *kbv1.Dep, val string) bool {
			return d.Idr == val
		})

	case kbv1.QueryDep_PARENT:
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

func (f *FileStore[T]) GetSotrsBy(ctx context.Context, query *kbv1.QuerySotr) (sotrs []*kbv1.Sotr, err error) {
	var s string

	f.mt.Lock()
	defer f.mt.Unlock()

	f.flS.Seek(0, io.SeekStart)
	field := query.Field

	sotrs = make([]*kbv1.Sotr, 0, 3)
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
				sotrs = append(sotrs, sotr)
			}
		}

		return
	}

	switch field {
	case kbv1.QuerySotr_EMPTY:
		err = findByFieldVal(func(d *kbv1.Sotr, val string) bool {
			return true
		})

	case kbv1.QuerySotr_IDR:
		err = findByFieldVal(func(d *kbv1.Sotr, val string) bool {
			return d.Idr == val
		})
	case kbv1.QuerySotr_MOBILE:
		err = findByFieldVal(func(d *kbv1.Sotr, val string) bool {
			return d.Mobile[0] == val
		})
	case kbv1.QuerySotr_FIO:
		err = findByFieldVal(func(d *kbv1.Sotr, val string) bool {
			return d.Name == val
		})
	case kbv1.QuerySotr_TABNUM:
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

func (f *FileStore[T]) GetDepByIdr1(ctx context.Context, idr *kbv1.QueryDep) (dep *kbv1.Dep, err error) {
	var s string
	f.mt.Lock()
	defer f.mt.Unlock()

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

func (f *FileStore[T]) GetSotrByTabnum(ctx context.Context, tabnum *kbv1.QuerySotr) (sotrs *kbv1.Sotr, err error) {
	var s string
	sComp := fmt.Sprintf("\"tabnum\":\"%s\",", tabnum.Str)
	sotrs = &kbv1.Sotr{}

	for err != io.EOF {
		s, err = f.rwrSotr.ReadString('\n')

		if err != nil {
			return
		}

		if strings.Contains(s, sComp) {
			err = json.Unmarshal([]byte(s), sotrs)

			if err != nil {
				return
			}
			break
		}
	}

	return
}

func (f *FileStore[T]) GetSotrByField(ctx context.Context, field string, quuery *kbv1.QuerySotr) (sotrs *kbv1.Sotr, err error) {
	var (
		s          string
		fieldValue string
	)

	sotrs = &kbv1.Sotr{}

	for err != io.EOF {
		s, err = f.rwrSotr.ReadString('\n')
		if err != nil {
			return
		}

		err = json.Unmarshal([]byte(s), sotrs)
		if err != nil {
			f.Log.Error("GetSotrByField: unmurshall json", "error", err, "field", field, "json", s)
			continue
		}

		switch field {
		case "tabnum":
			fieldValue = sotrs.Tabnum
		case "fio":
			fieldValue = sotrs.Name
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

// // Migrate apply migrations to the DB
// func (f *FileStore[T]) Migrate(ctx context.Context, down bool) error {
// 	return nil
// }

// // Retention deletes entries older than passed time
// func (f *FileStore[T]) Retention(ctx context.Context, olderThan time.Time) error {
// 	return nil
// }

func (f *FileStore[T]) PromCollector() prometheus.Collector {
	return nil
}

// func Save(ctx context.Context, query *kbv1.Item) (empty *emptypb.Empty, err error) {
// 	empty = &emptypb.Empty{}

// 	switch item := query.Var.(type) {
// 	case *kbv1.Item_Dep:
// 		var dep []*kbv1.Dep

// 		// get dep if exists for define double raw
// 		dep, err = ps.stor.GetDepsBy(ctx, &kbv1.QueryDep{Field: kbv1.QueryDep_IDR, Str: item.Dep.Idr})
// 		if err != nil {
// 			return
// 		}

// 		// check double raw
// 		for _, d := range dep {
// 			if d.Parent == item.Dep.Parent && d.Text == item.Dep.Text {
// 				ps.lg.Debug("Save GetDepBy: Dep exist", "IDR", item.Dep.Idr, "dep", dep)
// 				return
// 			}
// 		}
// 		ps.lg.Debug("Save Dep to Stor:", "item.dep", item.Dep)

// 		_, err = ps.stor.Save(ctx, item.Dep)
// 		if err != nil {
// 			return
// 		}

// 	case *kbv1.Item_Sotr:
// 		var sotrs []*kbv1.Sotr

// 		// get sotrs if exists for define double raw
// 		sotrs, err = ps.stor.GetSotrsBy(ctx, &kbv1.QuerySotr{Field: kbv1.QuerySotr_TABNUM, Str: item.Sotr.Tabnum})
// 		if err != nil {
// 			return
// 		}

// 		// doublicates not found
// 		if len(sotrs) == 0 {
// 			_, err = ps.stor.Save(ctx, item.Sotr)
// 			ps.lg.Debug("Save Sotr to Stor:", "item.sotrs", item.Sotr)
// 			if err != nil {
// 				err = fmt.Errorf("error save Sotr to Stor: %w", err)
// 				return
// 			}
// 			return
// 		}

// 		ps.lg.Debug("Save GetDepBy: Sotr exist", "TABNUM", item.Sotr.Tabnum, "sotrs", sotrs)

// 		sort.Slice(sotrs, func(i, j int) bool {
// 			if sotrs[i].Date == nil {
// 				return true
// 			}
// 			if sotrs[j].Date == nil {
// 				return false
// 			}
// 			return sotrs[i].Date.AsTime().Before(sotrs[j].Date.AsTime())
// 		})
// 		// get newest raw
// 		oldSotr := sotrs[len(sotrs)-1]

// 		// if double raw exists then compare for define difference
// 		diff, _ := kbv1.CompareSotr(oldSotr, item.Sotr)
// 		// if diff exists than save diff to history and update old sotrs
// 		if len(diff) > 0 {
// 			hs := make([]*kbv1.History, 0)
// 			for _, df := range diff {

// 				value := ""

// 				switch t := df.Val.(type) {
// 				case string:
// 					value = string(t)
// 				case []string:
// 					value = strings.Join(t, ",")
// 				}

// 				hs = append(hs, &kbv1.History{
// 					Date:     timestamppb.Now(),
// 					Field:    df.FieldName,
// 					OldValue: value,
// 					SotrId:   oldSotr.Id,
// 				})
// 			}
// 			_, err = ps.stor.Update(ctx, &kbv1.QueryUpdateSotr{Sotr: item.Sotr, HistoryList: hs})
// 			ps.lg.Debug("Update Sotr to Stor:", "item.sotrs", item.Sotr)
// 		} else {
// 			ps.lg.Debug("Sotr exist in Stor:", "item.sotrs", item.Sotr)
// 		}

// 	default:
// 		err = fmt.Errorf("can't save invalid query %v", query)
// 	}

// 	ps.lg.Debug("Saved item", "item", query.Var, "err", err)
// 	return
// }
