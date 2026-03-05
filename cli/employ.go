package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mioxin/kbempgo/internal/dump"
	"github.com/mioxin/kbempgo/internal/storage"
)

const ()

type dumpCommand struct {
	// Glob *config.Globals
	// KbUrl      string            `name:"scrape-url" placeholder:"URL" help:"Base Url"`
	// UrlRazd    string            `name:"scrape-razd" env:"KB_URL_RAZD" help:"Url of section"`
	// UrlSotr    string            `name:"scrape-sotr" env:"KB_URL_SOTR" help:"Url of employer"`
	// UrlFio     string            `name:"scrape-fio" env:"KB_URL_FIO" help:"Url of employer full nane"`
	// UrlMobile  string            `name:"scrape-mobil" env:"KB_URL_MOBIL" help:"Url of employer mobile"`
	// Avatars    string            `name:"scrape-avatars" env:"KB_AVATARS" help:"Directory for avatar images"`
	Workers  int    `name:"workers" short:"w" default:"5" env:"KB_WORKERS" help:"Number of workers. Every worker run 3 goroutines."`
	Limit    int    `name:"limit" short:"l" default:"0" env:"KB_LIMIT" help:"Limit of data for get. If =0 then no limit."`
	RootRazd string `name:"rootr" env:"KB_ROOT_RAZD" help:"Name of root section"`
	// FileSource string `name:"file_source" default:"" help:"Path includes dep.json and sotr.json for insert data from ones into storage"`
	// Grpc       gsrv.ServerConfig `embed:"" json:"grpc" prefix:"grpc-"`

	// grpcClient  *grpc.ClientConn `kong:"-"`
	Lg *slog.Logger `kong:"-"`
	// DepsCounter atomic.Int32 `kong:"-"`
	// SotrCounter atomic.Int32 `kong:"-"`
}

func (e *dumpCommand) Run(cli *CLI) error {
	var err error

	if e.Workers <= 0 {
		return fmt.Errorf("number of workers should be > 0")
	}

	ctx, cancel := context.WithTimeout(context.Background(), cli.OpTimeout)
	defer cancel()

	e.Lg = cli.Log.With("cmd", "employ")
	// ********************
	// dump data from url
	// ********************

	// open storage
	cli.Store, err = storage.NewStore(cli.StorageURL, e.Lg)
	if err != nil {
		return fmt.Errorf("create storage %w", err)
	}

	defer func() {
		cli.Log.Info("MAIN Close storage")
		if cli.Store != nil {
			if err := cli.Store.Close(); err != nil {
				cli.Log.Error("MAIN close storage", "err", err)
			}
		}
	}()

	sotrCounter := 0
	depsCounter := 0
	itemsCh := dump.StartDump(ctx, cancel, &dump.Config{Config: cli.Config,
		Workers:         e.Workers,
		Limit:           e.Limit,
		RootRazd:        e.RootRazd,
		OpTimeout:       cli.OpTimeout,
		WaitDataTimeout: cli.WaitDataTimeout,
		Debug:           cli.Debug,
		Lg:              cli.Log.With("cmd", "dump"),
	})
LOOP:
	for {
		select {
		case item, ok := <-itemsCh:
			if !ok {
				break LOOP
			}
			e.Lg.Debug("MAIN Save.", "item", item, "sotrs", sotrCounter, "Deps", depsCounter)

			if item.GetChildren() {
				depsCounter++
			} else {
				sotrCounter++
			}
			_, err := cli.Store.Save(ctx, item)
			if err != nil {
				e.Lg.Error("save item", "error", err)
				continue
			}

		case <-ctx.Done():
			break LOOP
		}
	}

	e.Lg.Info("MAIN Collected.", "SotrResponse", sotrCounter, "DepsResponse", depsCounter)
	return err
}
