package worker

import (
	"time"

	"github.com/mioxin/kbempgo/internal/storage"
)

type Config struct {
	KbUrl          string        `name:"scrape-url" placeholder:"URL" help:"Base Url"`
	UrlRazd        string        `name:"scrape-razd" env:"KB_URL_RAZD" help:"Url of section"`
	UrlSotr        string        `name:"scrape-sotr" env:"KB_URL_SOTR" help:"Url of employer"`
	UrlFio         string        `name:"scrape-fio" env:"KB_URL_FIO" help:"Url of employer full nane"`
	UrlMobile      string        `name:"scrape-mobil" env:"KB_URL_MOBIL" help:"Url of employer mobile"`
	Avatars        string        `name:"scrape-avatars" env:"KB_AVATARS" help:"Directory for avatar images"`
	HttpReqTimeout time.Duration `name:"req-timeout" default:"10s" help:"Http request timeout for worker"`
	// WaitDataTimeout time.Duration `name:"wait-timeout" default:"20s" help:"timeout for waiting data in dispatcher of worker"`

	Store storage.Store `kong:"-"`
}
