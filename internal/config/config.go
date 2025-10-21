package config

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/imroc/req/v3"
	"github.com/mioxin/kbempgo/internal/storage"
)

type HttpClientPool interface {
	//  chanal implementation
	//	Get(int) *req.Client
	//	Push(*req.Client) error

	//  sync.Pool implementation
	Get() *req.Client
	Push(*req.Client)
}

type Globals struct {
	KbUrl           string          `name:"scrape-url" placeholder:"URL" help:"Base Url"`
	ConfigFile      kong.ConfigFlag `name:"config-file" short:"c" type:"existingfile" help:"Config file location"`
	OpTimeout       time.Duration   `name:"op-timeout" default:"1600s" help:"timeout for Main getting"`
	HttpReqTimeout  time.Duration   `name:"req-timeout" default:"10s" help:"Http request timeout for worker"`
	WaitDataTimeout time.Duration   `name:"wait-timeout" default:"20s" help:"timeout for waiting data in dispatcher of worker"`
	Debug           int             `name:"scrape-debug" short:"d" type:"counter" help:"Enable debug"`
	UrlRazd         string          `name:"scrape-razd" env:"KB_URL_RAZD" help:"Url of section"`
	UrlSotr         string          `name:"scrape-sotr" env:"KB_URL_SOTR" help:"Url of employer"`
	UrlFio          string          `name:"scrape-fio" env:"KB_URL_FIO" help:"Url of employer full nane"`
	UrlMobile       string          `name:"scrape-mobil" env:"KB_URL_MOBIL" help:"Url of employer mobile"`
	Avatars         string          `name:"scrape-avatars" env:"KB_AVATARS" help:"Directory for avatar images"`
	DbUrl           string          `name:"db" env:"KB_DB_URL" help:"DB connection string"`

	lgInitOnce sync.Once          `kong:"-"`
	Log        *slog.Logger       `kong:"-"`
	Ctx        context.Context    `kong:"-"`
	Cf         context.CancelFunc `kong:"-"`
	Store      storage.Store      `kong:"-"`
	// ClientsPool HttpClientPool     `kong:"-"`
	ClientsPool *req.Client `kong:"-"`
}

// Done must be called on exit via defer
func (gl *Globals) Done() {
	if gl.Cf != nil {
		gl.Cf()
	}
}

// Context returns global context with --op-timeout
func (gl *Globals) Context() context.Context {
	if gl.Ctx == nil {
		gl.Ctx, gl.Cf = context.WithTimeout(context.Background(), gl.OpTimeout)
	}

	return gl.Ctx
}

func (gl *Globals) InitLog() {
	gl.lgInitOnce.Do(func() {
		gl.Log = slog.Default()
		if gl.Debug > 0 {
			slog.SetLogLoggerLevel(slog.LevelDebug)
		}
	})
}
