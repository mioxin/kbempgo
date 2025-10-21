package cors

import (
	"log/slog"

	"github.com/mioxin/kbempgo/pkg/logger"
	"github.com/rs/cors"
)

// Config options for CORS protection on gateway proxy
type Config struct {
	Enabled bool `name:"enabled" json:"enabled" default:"false" help:"CORS processing enabled"`

	AllowedOrigins   []string `name:"allowed-origins" json:"allowed-origins" default:"*" help:""`
	AllowedMethods   []string `name:"allowed-methods" json:"allowed-methods" default:"GET,POST" help:""`
	AllowedHeaders   []string `name:"allowed-headers" json:"allowed-headers" default:"Content-Type,Accept,X-Auth-Token" help:""`
	ExposedHeaders   []string `name:"exposed-headers" json:"exposed-headers" default:""`
	MaxAge           int      `name:"max-age" json:"max-age" default:"0" help:""`
	AllowCredentials bool     `name:"allow-credentials" json:"allow-credentials" negatable:"" default:"false" help:""`
	Debug            bool     `name:"debug" json:"debug" negatable:"" help:"Enable CORS request logging"`
}

func (s *Config) New() *cors.Cors {
	if !s.Enabled {
		return nil
	}

	corsCfg := cors.Options{
		AllowedOrigins:   s.AllowedOrigins,
		AllowedMethods:   s.AllowedMethods,
		AllowedHeaders:   s.AllowedHeaders,
		ExposedHeaders:   s.ExposedHeaders,
		MaxAge:           s.MaxAge,
		AllowCredentials: s.AllowCredentials,
	}

	cors := cors.New(corsCfg)

	if s.Debug {
		corsLog := logger.NewPrinterAt(slog.Default(), slog.LevelDebug)
		cors.Log = corsLog
	}

	return cors
}
