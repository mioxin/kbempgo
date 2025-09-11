package file

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mioxin/kbempgo/internal/models"
)

// string of directory path contains deps.json and sotr.json
type FileStore[T models.Item] struct {
	BaseDir       string
	wrDep, wrSotr *bufio.Writer
	flD, flS      *os.File
	// mt            sync.Mutex
}

func NewFileStore[T models.Item](fname string) (*FileStore[T], error) {

	err := os.MkdirAll(fname, 0750)
	if err != nil {
		return nil, err
	}

	fPath := filepath.Join(string(fname), "dep.json")
	flD, err := os.OpenFile(fPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	fPath = filepath.Join(string(fname), "sotr.json")
	flS, err := os.OpenFile(fPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	return &FileStore[T]{
		BaseDir: fname,
		wrDep:   bufio.NewWriter(flD),
		wrSotr:  bufio.NewWriter(flS),
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

	// f.mt.Lock()
	// defer f.mt.Unlock()

	if item.IsSotr() {
		_, err = f.wrSotr.Write(b)
		slog.Debug("sotr", "item", item)
	} else {
		_, err = f.wrDep.Write(b)
		slog.Debug("dep", "item", item)
	}

	return
}

func (f *FileStore[T]) Close() (err error) {
	f.wrDep.Flush()
	f.wrSotr.Flush()
	err = f.flD.Close()
	err1 := f.flS.Close()
	if err1 != nil {
		err = fmt.Errorf("%w; %w", err, err1)
	}
	return
}
