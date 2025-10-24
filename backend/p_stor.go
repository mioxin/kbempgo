package backend

import (
	"context"
	"fmt"
	"log/slog"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/types/known/emptypb"
)

// PStor implements persistent storage.Stor
type PStor struct {
	kbv1.UnimplementedStorServer
	stor    storage.Store
	lg      *slog.Logger
	dbmetrx prometheus.Collector
}

// Creating persistent storage
func NewPStor(cfg *CLI) (*PStor, error) {
	lg := cfg.Log.With("srv", "storage")

	if cfg.DbUrl == "" {
		lg.Info("Storage URL not configured", "url", cfg.DbUrl)
		return nil, fmt.Errorf("storage URL not configured, url= \"%s\"", cfg.DbUrl)
	}
	s, err := storage.NewStore(cfg.DbUrl)
	if err != nil {
		return nil, err
	}

	return &PStor{
		stor:    s,
		lg:      lg,
		dbmetrx: s.PromCollector(),
	}, nil
}

// gRPC implementation
func (ps *PStor) GetDepsBy(ctx context.Context, query *kbv1.QueryDep) (*kbv1.Deps, error) {
	d, err := ps.stor.GetDepsBy(ctx, query)
	return &kbv1.Deps{Deps: d}, err
}

func (ps *PStor) GetSotrsBy(ctx context.Context, query *kbv1.QuerySotr) (*kbv1.Sotrs, error) {
	s, err := ps.stor.GetSotrsBy(ctx, query)
	return &kbv1.Sotrs{Sotrs: s}, err
}

func (ps *PStor) Save(ctx context.Context, query *kbv1.Item) (empty *emptypb.Empty, err error) {
	switch item := query.Var.(type) {
	case *kbv1.Item_Dep:
		var dep []*kbv1.Dep

		// get dep if exists for define double raw
		dep, err = ps.stor.GetDepsBy(context.Background(), &kbv1.QueryDep{Field: kbv1.QueryDep_IDR, Str: item.Dep.Idr})
		if err != nil {
			return
		}

		// check double raw
		isDouble := false
		for _, d := range dep {
			if d.Parent == item.Dep.Parent && d.Text == item.Dep.Text {
				isDouble = true
				break
			}
		}

		if !isDouble {
			_, err = ps.stor.Save(ctx, item.Dep)
		}

	case *kbv1.Item_Sotr:
		var sotr []*kbv1.Sotr

		// get sotr if exists for define double raw
		sotr, err = ps.stor.GetSotrsBy(context.Background(), &kbv1.QuerySotr{Field: kbv1.QuerySotr_IDR, Str: item.Sotr.Idr})
		if err != nil {
			return
		}

		// if double raw exists then compare for define difference

		// if diff exists than save diff to history and update old sotr

		_, err = ps.stor.Save(ctx, item.Sotr)
	default:
		err = fmt.Errorf("can't save invalid query %v", query)
	}

	fmt.Println(err)
	return
}

func (ps *PStor) Close() error {
	return ps.stor.Close()
}
