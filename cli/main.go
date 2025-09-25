package cli

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/mioxin/kbempgo/internal/clientpool"
	"github.com/mioxin/kbempgo/internal/storage"
	"github.com/mioxin/kbempgo/pkg/kongyaml"
)

type Globals struct {
	KbUrl           string          `name:"url" placeholder:"URL" help:"Base Url"`
	ConfigFile      kong.ConfigFlag `name:"config-file" short:"c" type:"existingfile" help:"Config file location"`
	OpTimeout       time.Duration   `name:"op-timeout" default:"1600s" help:"timeout for Main getting"`
	HttpReqTimeout  time.Duration   `name:"req-timeout" default:"10s" help:"Http request timeout for worker"`
	WaitDataTimeout time.Duration   `name:"wait-timeout" default:"20s" help:"timeout for waiting data in dispatcher of worker"`
	Debug           int             `name:"debug" short:"d" type:"counter" help:"Enable debug"`
	UrlRazd         string          `name:"razd" env:"KB_URL_RAZD" help:"Url of section"`
	UrlSotr         string          `name:"sotr" env:"KB_URL_SOTR" help:"Url of employer"`
	UrlFio          string          `name:"fio" env:"KB_URL_FIO" help:"Url of employer full nane"`
	DbUrl           string          `name:"db" env:"KB_DB_URL" help:"DB connection string"`
	Avatars         string          `name:"avatars" env:"KB_AVATARS" help:"Directory for avatar images"`

	lgInitOnce  sync.Once          `kong:"-"`
	log         *slog.Logger       `kong:"-"`
	ctx         context.Context    `kong:"-"`
	cf          context.CancelFunc `kong:"-"`
	store       storage.Store      `kong:"-"`
	clientsPool HttpClientPool     `kong:"-"`
}

// Done must be called on exit via defer
func (gl *Globals) Done() {
	if gl.cf != nil {
		gl.cf()
	}
}

// Context returns global context with --op-timeout
func (gl *Globals) Context() context.Context {
	if gl.ctx == nil {
		gl.ctx, gl.cf = context.WithTimeout(context.Background(), gl.OpTimeout)
	}
	return gl.ctx
}

func (gl *Globals) InitLog() {
	gl.lgInitOnce.Do(func() {
		gl.log = slog.Default()
		if gl.Debug > 0 {
			slog.SetLogLoggerLevel(slog.LevelDebug)
		}
	})

}

// CLI all commands
type CLI struct {
	Globals

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
	cli.Globals.clientsPool = clientpool.NewClientPool(cli.Globals.Debug)

	cli.Globals.store, err = storage.NewStore(cli.DbUrl)
	kctx.FatalIfErrorf(err, "create file storage")

	defer func() {
		cli.Globals.log.Info("Close storage")
		if err := cli.Globals.store.Close(); err != nil {
			kctx.FatalIfErrorf(err)
		}
	}()

	err = kctx.Run(&cli.Globals)
	kctx.FatalIfErrorf(err)
}
