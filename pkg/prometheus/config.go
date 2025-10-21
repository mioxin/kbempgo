package prometheus

import (
	"github.com/mcuadros/go-defaults"
)

// ClientConfig represents configuration for prometheus clinet
type ClientConfig struct {
	DBURL         string `json:"db_url" name:"db-url" help:"Endpoint URL"`                                  // prometheus api url
	MetadataDBURL string `json:"metadata_db_url,omitempty" name:"metadata-db-url" help:"Metadata URL"`      // if set that url would be used to query metadata
	Username      string `json:"username,omitempty" name:"username" help:"Username"`                        // optional login for basic auth (XXX TODO)
	Password      string `json:"password,omitempty" name:"password" help:"Password"`                        // optional password for basic auth (TODO)
	CacheTTLSecs  int    `json:"cache_ttl_secs" default:"60" name:"cache-ttl-secs" help:"Cache TTL [secs]"` //
}

// SetDefault applies default values
func (c *ClientConfig) SetDefaults() {
	defaults.SetDefaults(c)
}
