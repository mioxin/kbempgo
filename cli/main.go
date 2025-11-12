package cli

import (
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/imroc/req/v3"
	"github.com/mioxin/kbempgo/internal/config"
	"github.com/mioxin/kbempgo/internal/worker"
	"github.com/mioxin/kbempgo/pkg/kongyaml"
)

// CLI all commands
type CLI struct {
	worker.Config
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

	kctx := kong.Parse(cli,
		kong.Description("Update kbEmp data base cli tool"),
		kong.Configuration(kongyaml.Loader, "/etc/kbemp/kb.yaml", "~/.config/kb.yaml"),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: false,
		}),
		kong.DefaultEnvars("KB"),
	)

	cli.HttpClientPool = make(map[string]*req.Client, 5)
	cli.InitLog()

	err = kctx.Run(&cli)
	kctx.FatalIfErrorf(err)
}
