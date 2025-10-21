package cli

import (
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/mioxin/kbempgo/internal/config"
	httpclient "github.com/mioxin/kbempgo/internal/http_client"
	"github.com/mioxin/kbempgo/internal/storage"
	"github.com/mioxin/kbempgo/pkg/kongyaml"
)

// CLI all commands
type CLI struct {
	config.Globals

	Employes employCommand `cmd:"" aliases:"empl" help:"Update or get employes DB"`
	News     newsCommand   `cmd:"" aliases:"news" help:"Get news and comments."`
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

	err = kctx.Run(&cli.Globals)
	kctx.FatalIfErrorf(err)
}
