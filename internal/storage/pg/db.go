package pg

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PgStore struct {
	kbv1.UnimplementedStorServer

	DB  *gorm.DB
	Log *slog.Logger
}

func New(dsn string, log *slog.Logger) (pgs *PgStore, err error) {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		err = fmt.Errorf("error create Store, invalid source, %s. %w", dsn, err)
		return
	}
	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})

	if err != nil {
		err = fmt.Errorf("error create Store, invalid source, %s. %w", dsn, err)
		return
	}

	return &PgStore{
		DB:  db,
		Log: log.With("storage", "postres"),
	}, nil
}

func (p *PgStore) GetDepsBy(ctx context.Context, q *kbv1.QueryDep) (i []*kbv1.Dep, err error) {
	return
}

// GetSotr returns employee data
func (p *PgStore) GetSotrsBy(ctx context.Context, q *kbv1.QuerySotr) (i []*kbv1.Sotr, err error) {
	return
}

// Save Item data
func (p *PgStore) Save(ctx context.Context, item models.Item) (em *emptypb.Empty, err error) {
	// **************************
	// save Dep
	// **************************
	// get dep by idr
	// if exists return
	// else

	// **************************
	// save Sotr
	// **************************
	return
}

func (p *PgStore) Update(ctx context.Context, q *kbv1.QueryUpdateSotr) (em *emptypb.Empty, err error) {
	return
}

func (p *PgStore) Close() (err error) {
	return
}
func (p *PgStore) Flush() (err error) {
	return
}

func (p *PgStore) PromCollector() (prom prometheus.Collector) {
	return
}

// Migrate apply migrations to the DB
func (p *PgStore) Migrate(ctx context.Context, down bool) (err error) {
	return
}

// Retention deletes entries older than passed time
func (p *PgStore) Retention(ctx context.Context, olderThan time.Time) (err error) {
	return
}
