package backend

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	empty = &emptypb.Empty{}

	switch item := query.Var.(type) {
	case *kbv1.Item_Dep:
		var dep []*kbv1.Dep

		// get dep if exists for define double raw
		dep, err = ps.stor.GetDepsBy(ctx, &kbv1.QueryDep{Field: kbv1.QueryDep_IDR, Str: item.Dep.Idr})
		if err != nil {
			return
		}

		// check double raw
		for _, d := range dep {
			if d.Parent == item.Dep.Parent && d.Text == item.Dep.Text {
				ps.lg.Debug("Save GetDepBy: Dep exist", "IDR", item.Dep.Idr, "dep", dep)
				return
			}
		}
		ps.lg.Debug("Save Dep to Stor:", "item.dep", item.Dep)

		_, err = ps.stor.Save(ctx, item.Dep)
		if err != nil {
			return
		}

	case *kbv1.Item_Sotr:
		var sotr []*kbv1.Sotr

		// get sotr if exists for define double raw
		sotr, err = ps.stor.GetSotrsBy(ctx, &kbv1.QuerySotr{Field: kbv1.QuerySotr_TABNUM, Str: item.Sotr.Tabnum})
		if err != nil {
			return
		}

		// doublicates not found
		if len(sotr) == 0 {
			_, err = ps.stor.Save(ctx, item.Sotr)
			ps.lg.Debug("Save Sotr to Stor:", "item.sotr", item.Sotr)
			if err != nil {
				err = fmt.Errorf("error save Sotr to Stor: %w", err)
				return
			}
			return
		}

		ps.lg.Debug("Save GetDepBy: Sotr exist", "TABNUM", item.Sotr.Tabnum, "sotr", sotr)

		sort.Slice(sotr, func(i, j int) bool {
			if sotr[i].Date == nil {
				return true
			}
			if sotr[j].Date == nil {
				return false
			}
			return sotr[i].Date.AsTime().Before(sotr[j].Date.AsTime())
		})
		// get newest raw
		oldSotr := sotr[len(sotr)-1]

		// if double raw exists then compare for define difference
		diff, _ := kbv1.CompareSotr(oldSotr, item.Sotr)
		// if diff exists than save diff to history and update old sotr
		if len(diff) > 0 {
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
			_, err = ps.stor.Update(ctx, &kbv1.QueryUpdateSotr{Sotr: item.Sotr, HistoryList: hs})
			ps.lg.Debug("Update Sotr to Stor:", "item.sotr", item.Sotr)
		} else {
			ps.lg.Debug("Sotr exist in Stor:", "item.sotr", item.Sotr)
		}

	default:
		err = fmt.Errorf("can't save invalid query %v", query)
	}

	ps.lg.Debug("Saved item", "item", query.Var, "err", err)
	return
}

func (ps *PStor) Flush(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, ps.stor.Flush()
}

func (ps *PStor) Close() error {
	return ps.stor.Close()
}
