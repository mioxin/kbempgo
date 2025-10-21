package backend

import (
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/mioxin/kbempgo/internal/config"
	httpclient "github.com/mioxin/kbempgo/internal/http_client"
	"github.com/mioxin/kbempgo/internal/storage"
	"github.com/mioxin/kbempgo/pkg/kongyaml"
)

type CLI struct {
	Config
	config.Globals

	Start  struct{} `cmd:"" name:"start" default:"1" help:"Start kbempgo backend service"`
	DBSync struct{} `cmd:"" name:"dbsync" help:"DB init and migration."`
}

// Main CLI func
func Main() {
	var err error
	// defer zap.S().Sync() // nolint
	start := time.Now()

	defer func() {
		fmt.Println("Time:", time.Since(start))
	}()

	cli := &CLI{}
	defer cli.Done()

	cli.InitLog()
	cli.Context()

	kctx := kong.Parse(cli,
		kong.Description("Update kbEmp data base cli tool"),
		kong.Configuration(kongyaml.Loader, "/etc/kbemp/kb.yaml", "~/.config/kb.yaml"),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: false,
		}),
		kong.DefaultEnvars("KB"),
	)

	cli.ClientsPool = httpclient.NewHTTPClient(cli.Debug)

	cli.Store, err = storage.NewStore(cli.DbUrl)
	kctx.FatalIfErrorf(err, "create file storage")

	defer func() {
		cli.Log.Info("Close storage")

		if err := cli.Store.Close(); err != nil {
			kctx.FatalIfErrorf(err)
		}
	}()

	err = startGrpc(cli)

	kctx.FatalIfErrorf(err)
}
