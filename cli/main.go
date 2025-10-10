package cli

import (
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/mioxin/kbempgo/internal/clientpool"
	"github.com/mioxin/kbempgo/internal/config"
	"github.com/mioxin/kbempgo/internal/storage"
	"github.com/mioxin/kbempgo/pkg/kongyaml"
)

// CLI all commands
type CLI struct {
	config.Globals

	Employes employCommand `cmd:"" aliases:"empl" help:"Update or get employes DB"`
	News     newsCommand   `cmd:"" aliases:"news" help:"Get news and comments."`
}

func Main() {
	var err error
	// defer zap.S().Sync() // nolint
	start := time.Now()
	defer func() {
		fmt.Println("Time:", time.Since(start))
	}()

	cli := &CLI{}
	defer cli.Done()

	kctx := kong.Parse(cli,
		kong.Description("Update kbEmp data base cli tool"),
		kong.Configuration(kongyaml.Loader, "/etc/kbemp/kb.yaml", "~/.config/kb.yaml"),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: false,
		}),
		kong.DefaultEnvars("KB"),
	)

	//	clientsPool := clientpool.NewClientsPool(e.Workers)
	cli.Globals.ClientsPool = clientpool.NewClientPool(cli.Globals.Debug)

	cli.Globals.Store, err = storage.NewStore(cli.DbUrl)
	kctx.FatalIfErrorf(err, "create file storage")

	defer func() {
		cli.Globals.Log.Info("Close storage")
		if err := cli.Globals.Store.Close(); err != nil {
			kctx.FatalIfErrorf(err)
		}
	}()

	err = kctx.Run(&cli.Globals)
	kctx.FatalIfErrorf(err)
}
