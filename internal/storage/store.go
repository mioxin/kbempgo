package storage

import (
	"fmt"
	"strings"

	"github.com/mioxin/kbempgo/internal/models"
	"github.com/mioxin/kbempgo/internal/storage/file"
)

type Store interface {
	Save(item models.Item) error
	Close() error
}

func NewStore(source string) (st Store, err error) {
	if source == "" {
		return nil, fmt.Errorf("error create Store, source is empty")
	}
	dbType := strings.Split(source, ":")[0]
	switch dbType {
	case "postgresql":
		// todo: create db storage struct
	case "file":
		s, ok := strings.CutPrefix(source, "file://")
		if !ok {
			err = fmt.Errorf("error create Store, invalid source, 'file://' not found (%s)", source)
			break
		}
		st, err = file.NewFileStore[models.Item](s)
	default:
		err = fmt.Errorf("error create Store, invalid db type in the source \"%v\" (%s)", dbType, source)
	}
	return st, err
}
