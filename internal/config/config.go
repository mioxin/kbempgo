package config

import (
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/imroc/req/v3"
)

type Globals struct {
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
		opts := &slog.HandlerOptions{}
		out := os.Stdout

		if gl.LogOutput != "" {
			f, err := os.OpenFile(gl.LogOutput, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
			if err != nil {
				slog.Error("error open log file", "file", gl.LogOutput, "err", err)
			} else {
				out = f
			}
		}

		if gl.Debug > 0 {
			opts.Level = slog.LevelDebug
		}

		if gl.Debug > 2 {
			opts.AddSource = true
		}

		if gl.JsonLog {
			gl.Log = slog.New(slog.NewJSONHandler(out, opts))
		} else {
			gl.Log = slog.New(slog.NewTextHandler(out, opts))
		}

		// TODO: add handler for loki and ELK
	})
}
