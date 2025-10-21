package grpc_client

import (
	"time"

	"github.com/mcuadros/go-defaults"
	"github.com/mioxin/kbempgo/pkg/tlsutil"
)

// SSHProxyConfig
type SSHProxyConfig struct {
	Host    string `name:"host" json:"host" help:"SSH hostname"`
	Port    int    `name:"port" json:"port" default:"22" help:"SSH port (default 22)"`
	User    string `name:"user" json:"user" help:"Remove user name"`
	Verbose bool   `name:"verbose" json:"verbose" negatable:"" default:"false" help:"Enable verbose ssh client logging"`
}

// ClientConfig section of gRPC client config
type ClientConfig struct {
	tlsutil.TLSConfig `embed:"" yaml:",inline"`

	Address              string         `name:"address" json:"address" help:"Remote address"`
	Endpoints            []string       `name:"endpoint" json:"endpoints" help:"Remote endpoint (same as address)"` // XXX TODO(vermakov)
	DialTimeout          time.Duration  `name:"dial-timeout" json:"dial-timeout" default:"10s" help:"Dial timeout"`
	DialKeepAliveTime    time.Duration  `name:"dial-keepalive-time" json:"dial-keepalive-time" help:"Keepalive time"`
	DialKeepAliveTimeout time.Duration  `name:"dial-keepalive-timeout" json:"dial-keepalive-timeout" default:"10s" help:"Keepalive timeout"`
	PermitWithoutStream  bool           `name:"permit-without-stream" json:"permit-without-stream" negatable:"" default:"true" help:"Allow to connect to server wihout stream support"`
	SSHProxy             SSHProxyConfig `embed:"" prefix:"ssh-proxy-" json:"ssh-proxy,omitempty" help:"Use SSH tunneling for connection"`
}

// SetDefaults apply defaults
func (gclic *ClientConfig) SetDefaults() {
	defaults.SetDefaults(gclic)
}

// GetEndpoints XXX TODO
func (gclic *ClientConfig) GetEndpoints() []string {
	if len(gclic.Endpoints) > 0 {
		return gclic.Endpoints
	}
	return []string{gclic.Address}
}

// GetAddress return first endpoint or address
func (gclic *ClientConfig) GetAddress() string {
	if len(gclic.Endpoints) > 0 {
		return gclic.Endpoints[0]
	}
	return gclic.Address
}
