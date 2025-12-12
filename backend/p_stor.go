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
	kbv1.UnimplementedStorAPIServer
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
	s, err := storage.NewStore(cfg.DbUrl, cfg.Log)
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
func (ps *PStor) GetDepsBy(ctx context.Context, query *kbv1.DepRequest) (*kbv1.DepsResponse, error) {
	d, err := ps.stor.GetDepsBy(ctx, query)
	return &kbv1.DepsResponse{Deps: d}, err
}

func (ps *PStor) GetSotrsBy(ctx context.Context, query *kbv1.SotrRequest) (*kbv1.SotrsResponse, error) {
	s, err := ps.stor.GetSotrsBy(ctx, query)
	return &kbv1.SotrsResponse{Sotrs: s}, err
}

func (ps *PStor) Save(ctx context.Context, query *kbv1.Item) (empty *emptypb.Empty, err error) {
	empty = &emptypb.Empty{}

	switch item := query.Var.(type) {
	case *kbv1.Item_Dep:
		_, err = ps.stor.Save(ctx, item.Dep)
		if err != nil {
			return
		}

	case *kbv1.Item_Sotr:
		_, err = ps.stor.Save(ctx, item.Sotr)
		if err != nil {
			return
		}

	default:
		err = fmt.Errorf("can't save invalid query %v", query)
	}

	if err == nil {
		ps.lg.Debug("Saved item", "item", query.Var)
	}

	return
}

func (ps *PStor) Flush(ctx context.Context, em *emptypb.Empty) (*emptypb.Empty, error) {
	return ps.stor.Flush(ctx, em)
}

func (ps *PStor) Close() error {
	return ps.stor.Close()
}
