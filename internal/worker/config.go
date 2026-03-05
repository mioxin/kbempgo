package worker

import (
	"time"

	"github.com/mioxin/kbempgo/internal/storage"
)

type Config struct {
	KbUrl            string        `name:"scrape-url" placeholder:"URL" help:"Base Url"`
	UrlRazd          string        `name:"scrape-razd" env:"KB_URL_RAZD" help:"Url of section"`
	UrlSotr          string        `name:"scrape-sotr" env:"KB_URL_SOTR" help:"Url of employer"`
	UrlFio           string        `name:"scrape-fio" env:"KB_URL_FIO" help:"Url of employer full name"`
	UrlMobile        string        `name:"scrape-mobil" env:"KB_URL_MOBIL" help:"Url of employer mobile"`
	Avatars          string        `name:"scrape-avatars" env:"KB_AVATARS" help:"Directory for avatar images"`
	HttpReqTimeout   time.Duration `name:"req-timeout" default:"6s" help:"Http request timeout for worker"`
	DispPollInterval time.Duration `name:"disp-pollinterval" default:"300ms" help:"Polling interval for getting data from dispatcher queue"`
	StorageURL       string        `name:"scrape-storage" env:"KB_STORAGE" help:"Storage connection string for scraped data. Example: postgres://localhost:5432/db, file:///home/user/dir"`

	Headers []string      `name:"scrape-headers" yaml:"headers" help:"Headers of http requsts as map[string]string in config file"`
	Store   storage.Store `kong:"-"`
}
