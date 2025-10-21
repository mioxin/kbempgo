package file

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/models"
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

	flD, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE, 0644)
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

func (f *FileStore[T]) Save(item T) (err error) {
	b, err := json.Marshal(item)
	if err != nil {
		return
	}

	b = append(b, "\n"...)

	f.mt.Lock()
	defer f.mt.Unlock()

	if item.IsSotr() {
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

func (f *FileStore[T]) GetDepByIdr(idr *kbv1.QueryString) (dep *models.Dep, err error) {
	var s string

	dep = &models.Dep{}
	for err != io.EOF {
		s, err = f.rwrDep.ReadString('\n')
		if err != nil {
			return
		}

		err = json.Unmarshal([]byte(s), dep)
		if err != nil {
			slog.Error("GetDepByIdr: unmurshall json", "error", err, "json", s)
			continue
		}

		if dep.Idr == idr.Str {
			break
		}
	}

	return
}

func (f *FileStore[T]) GetSotrByField(field string, quuery *kbv1.QueryString) (sotr *models.Sotr, err error) {
	var (
		s          string
		fieldValue string
	)

	sotr = &models.Sotr{}

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

func (f *FileStore[T]) GetSotrsByField(field string, quuery *kbv1.QueryString) (sotrs []*models.Sotr, err error) {
	var (
		s          string
		fieldValue string
	)

	sotrs = make([]*models.Sotr, 0, 3)

	for err != io.EOF {
		sotr := &models.Sotr{}
		s, err = f.rwrSotr.ReadString('\n')
		if err != nil {
			return
		}

		err = json.Unmarshal([]byte(s), sotr)
		if err != nil {
			slog.Error("GetSotrsByField: unmurshall json", "error", err, "field", field, "json", s)
			continue
		}

		switch field {
		case "mobile":
			fieldValue = sotr.Mobile
		default:
			// err = fmt.Errorf("invalid field name; field=%s", field)
			err = &FieldNameError{Name: field}
			return
		}

		if fieldValue == quuery.Str {
			sotrs = append(sotrs, sotr)
		}
	}

	return
}

func (f *FileStore[T]) GetDepByIdr1(idr *kbv1.QueryString) (dep *models.Dep, err error) {
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

func (f *FileStore[T]) GetSotrByTabnum(tabnum *kbv1.QueryString) (sotr *models.Sotr, err error) {
	var s string
	sComp := fmt.Sprintf("\"tabnum\":\"%s\",", tabnum.Str)
	sotr = &models.Sotr{}

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
