package config

import (
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/imroc/req/v3"
)

type Globals struct {
	Env             string          `name:"env" env:"KB_ENV" enum:"local,dev,prod" default:"local" help:"App Enviroment: local, dev, prod"`
	ConfigFile      kong.ConfigFlag `name:"config-file" short:"c" type:"existingfile" help:"Config file location"`
	OpTimeout       time.Duration   `name:"op-timeout" default:"1600s" help:"timeout for Main getting"`
	WaitDataTimeout time.Duration   `name:"wait-timeout" default:"20s" help:"timeout for waiting data in dispatcher of worker"`
	Debug           int             `name:"debug" short:"d" type:"counter" help:"Enable debug"`
	DbUrl           string          `name:"db" env:"KB_DB_URL" help:"DB connection string"`
	LogOutput       string          `name:"log-output" short:"o" default:"" help:"output file for logs, default: stdOut"`
	JsonLog         bool            `name:"json" help:"set JSON format for logs"`

	lgInitOnce     sync.Once              `kong:"-"`
	Log            *slog.Logger           `kong:"-"`
	HttpClientPool map[string]*req.Client `kong:"-"`
}

func (gl *Globals) InitLog() {
	gl.lgInitOnce.Do(func() {

		var out io.Writer
		opts := &slog.HandlerOptions{}

		if gl.Debug > 0 {
			opts.Level = slog.LevelDebug
		}

		if gl.Debug > 2 {
			opts.AddSource = true
		}

		switch gl.Env {
		case "local":
			out = os.Stdout

			if gl.LogOutput != "" {
				f, err := os.OpenFile(gl.LogOutput, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
				if err != nil {
					slog.Error("error open log file", "file", gl.LogOutput, "err", err)
				} else {
					out = f
				}
			}
			gl.Log = slog.New(slog.NewTextHandler(out, opts))

		case "prod", "dev":
			// TODO: add handler for loki and ELK
			// out = Loki
			gl.Log = slog.New(slog.NewJSONHandler(out, opts))
		}

	})
}
