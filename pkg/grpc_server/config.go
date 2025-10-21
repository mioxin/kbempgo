package grpc_server

import (
	"time"

	"github.com/mcuadros/go-defaults"
	"github.com/mioxin/kbempgo/pkg/cors"
	"github.com/mioxin/kbempgo/pkg/grpc_client"
	"github.com/mioxin/kbempgo/pkg/tlsutil"
)

// ServerConfig section of gRPC server config
type ServerConfig struct {
	tlsutil.TLSConfig `embed:"" yaml:",inline"`

	Listen                      string        `name:"listen" json:"listen" help:"Listen address (host:port)"`
	Debug                       bool          `name:"debug" json:"debug" negatable:"" default:"false" help:"enable debug logging on gRPC"`
	KeepAliveEnforcementMinTime time.Duration `name:"keepalive-enforcement-min-time" json:"keepalive-enforcement-min-time" default:"60s"`
	KeepAliveTime               time.Duration `name:"keepalive-time" json:"keepalive-time" default:"10s"`
	KeepAliveTimeout            time.Duration `name:"keepalive-timeout" json:"keepalive-timeout" default:"20s"`
}

// ProxyConfig section for gRPC gateway proxy
type ProxyConfig struct {
	tlsutil.TLSConfig `embed:"" yaml:",inline"`

	Listen string      `name:"listen" json:"listen" help:"Listen address (host:port)"`
	CORS   cors.Config `embed:"" prefix:"cors-" json:"cors" help:"CORS settings"`
	UseXFF bool        `name:"use-x-forwarded-for" json:"use-x-forwarded-for" negatable:"" default:"true" help:"Process proxy X-Forwarded-For header"`
}

// SetDefaults apply defaults
func (grpcs *ServerConfig) SetDefaults() {
	defaults.SetDefaults(grpcs)
}

// ClientConfig makes config for gRPC client
func (grpcs *ServerConfig) ClientConfig() *grpc_client.ClientConfig {
	ret := &grpc_client.ClientConfig{}
	ret.SetDefaults()
	ret.TLSConfig = grpcs.TLSConfig
	ret.Address = grpcs.Listen
	return ret
}
