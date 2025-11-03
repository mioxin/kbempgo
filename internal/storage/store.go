package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/mioxin/kbempgo/internal/storage/file"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Store is persistent storage
type Store interface {
	GetDepsBy(context.Context, *kbv1.QueryDep) ([]*kbv1.Dep, error)
	// GetSotr returns employee data
	GetSotrsBy(context.Context, *kbv1.QuerySotr) ([]*kbv1.Sotr, error)
	// Save Item data
	Save(context.Context, models.Item) (*emptypb.Empty, error)

	Update(context.Context, *kbv1.QueryUpdateSotr) (*emptypb.Empty, error)
	// Save(item models.Item) error
	Close() error
	Flush() error

	// Migrate apply migrations to the DB
	Migrate(ctx context.Context, down bool) error

	// Retention deletes entries older than passed time
	Retention(ctx context.Context, olderThan time.Time) error

	PromCollector() prometheus.Collector
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
