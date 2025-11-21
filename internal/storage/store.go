package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/mioxin/kbempgo/internal/storage/file"
	"github.com/mioxin/kbempgo/internal/storage/pg"
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
	PromCollector() prometheus.Collector
}

type StoreManager interface {
	// Migrate apply migrations to the DB
	Migrate(ctx context.Context, down bool) error

	// Retention deletes entries older than passed time
	Retention(ctx context.Context, olderThan time.Time) error
}

func NewStore(source string, log *slog.Logger) (st Store, err error) {
	if source == "" {
		return nil, fmt.Errorf("error create Store, source is empty")
	}

	dbType := strings.Split(source, ":")[0]
	switch dbType {
	case "postgres":
		// todo: create db storage struct
		st, err = pg.New(source, log)
		if err != nil {
			err = fmt.Errorf("error create Store, invalid source, %s. %w", source, err)
			break
		}
	case "file":
		s, ok := strings.CutPrefix(source, "file://")
		if !ok {
			err = fmt.Errorf("error create Store, invalid source, 'file://' not found (%s)", source)
			break
		}

		st, err = file.NewFileStore[models.Item](s, log)
	default:
		err = fmt.Errorf("error create Store, invalid db type in the source \"%v\" (%s)", dbType, source)
	}

	return st, err
}
