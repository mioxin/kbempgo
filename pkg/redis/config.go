package redis

import (
	"time"

	"github.com/jinzhu/copier"
	"github.com/mcuadros/go-defaults"
	"github.com/redis/go-redis/v9"
)

// ClientConfig represents redis.UniversalOptions
type ClientConfig struct {
	Addrs []string `json:"addrs" name:"addrs" help:"Redis addresses"`
	DB    int      `json:"db" default:"0" name:"db" help:"Redis DB"`

	// Common options
	Username           string        `json:"username" name:"username" help:"username"`
	Password           string        `json:"password" name:"password" help:"password"`
	MaxRetries         int           `json:"max-retries" name:"max-retries" help:"max retries"`
	MinRetryBackoff    time.Duration `json:"min-retry-backoff" default:"8us" name:"min-retry-backoff" help:"min retry backoff"`
	MaxRetryBackoff    time.Duration `json:"max-retry-backoff" default:"512us" name:"max-retry-backoff" help:"max retry backoff"`
	DialTimeout        time.Duration `json:"dial-timeout" default:"30s" name:"dial-timeout" help:"dial timeout"`
	ReadTimeout        time.Duration `json:"read-timeout" default:"3s" name:"read-timeout" help:"read timeout"`
	WriteTimeout       time.Duration `json:"write-timeout" default:"3s" name:"write-timeout" help:"write timeout"`
	PoolSize           int           `json:"pool-size" name:"pool-size" help:"pool size"`
	MinIdleConns       int           `json:"min-idle-conns" name:"min-idle-conns" help:"min idle connections"`
	MaxConnAge         time.Duration `json:"max-conn-age" name:"max-conn-age" help:"max connection age"`
	PoolTimeout        time.Duration `json:"pool-timeout" name:"pool-timeout" help:"pool-timeout"`
	IdleTimeout        time.Duration `json:"idle-timeout" name:"idle-timeout" help:"idle-timeout"`
	IdleCheckFrequency time.Duration `json:"idle-check-frequency" name:"idle-check-frequency" help:"idle check frequency"`

	// Cluster only options
	MaxRedirects   int  `json:"max-redirects" default:"8" name:"max-redirects" help:"max redirects"`
	ReadOnly       bool `json:"read-only" default:"false" name:"read-only" help:"read only"`
	RouteByLatency bool `json:"route-by-latency" default:"false" name:"route-by-latency" help:"route by latency"`
	RouteRandomly  bool `json:"route-randomly" default:"false" name:"route-randomly" help:"route randomly"`

	// Sentinel only options
	MasterName       string `json:"sentinel-master-name" name:"sentinel-master-name" help:"Sentinel master name"`
	SentinelUsername string `json:"sentinel-username" name:"sentinel-username" help:"Sentinel user name"`
	SentinelPassword string `json:"sentinel-password" name:"sentinel-password" help:"Sentinel password"`

	// Client options
	OtelTracing bool `json:"otel-tracing" default:"true" name:"otel-tracing" negatable:"" help:"Enable OpenTelemetry tracing"`
}

// SetDefaults apply default values
func (c *ClientConfig) SetDefaults() {
	defaults.SetDefaults(c)
}

// UniversalOptions converts config into redis.UniversalOptions
func (c *ClientConfig) UniversalOptions() *redis.UniversalOptions {
	ret := &redis.UniversalOptions{}
	_ = copier.Copy(ret, c)

	return ret
}
