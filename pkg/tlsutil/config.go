// Package tlsutil provides helper to load tls certs
package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/mcuadros/go-defaults"
	"go.etcd.io/etcd/client/pkg/v3/tlsutil"
)

// TLSConfig defines settings for TLS-enabled server
type TLSConfig struct {
	InsecureTransport     bool   `name:"insecure-transport" json:"insecure-transport" negatable:"" default:"true" help:"Do not setup TLS"`
	InsecureSkipTLSVerify bool   `name:"insecure-skip-tls-verify" json:"insecure-skip-tls-verify" negatable:"" default:"false" help:"Skip TLS verify"`
	CertFile              string `name:"cert-file" json:"cert-file" default:"" help:"Certificate in x509"`
	KeyFile               string `name:"key-file" json:"key-file" default:"" help:"Private key"`
	TrustedCAFile         string `name:"trusted-ca-file" json:"trusted-ca-file" default:"" help:"Certificate Authority (replace the system one)"`
	ServerName            string `name:"server-name" json:"server-name" help:"Set server name for verification"`

	TLS *tls.Config `kong:"-" json:"-" yaml:"-"`
}

// SetDefaults apply defaults
func (tlsc *TLSConfig) SetDefaults() {
	defaults.SetDefaults(tlsc)
}

// Load loads TLS certificates if secure connection requested
func (tlsc *TLSConfig) Load() error {
	if tlsc.InsecureTransport {
		return nil
	}

	var (
		cert *tls.Certificate
		cp   *x509.CertPool
		err  error
	)

	if tlsc.CertFile != "" && tlsc.KeyFile != "" {
		cert, err = tlsutil.NewCert(tlsc.CertFile, tlsc.KeyFile, nil)
		if err != nil {
			return fmt.Errorf("New cert error: %w", err)
		}
	}

	if tlsc.TrustedCAFile != "" {
		cp, err = tlsutil.NewCertPool([]string{tlsc.TrustedCAFile})
		if err != nil {
			return fmt.Errorf("New cert pool error: %w", err)
		}
	} else {
		cp, err = x509.SystemCertPool()
		if err != nil {
			return fmt.Errorf("system cert pool error: %w", err)
		}
	}

	tlscfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: tlsc.InsecureSkipTLSVerify,
		RootCAs:            cp,
	}
	if cert != nil {
		tlscfg.Certificates = []tls.Certificate{*cert}
	}
	if tlsc.ServerName != "" {
		tlscfg.ServerName = tlsc.ServerName
	}

	tlsc.TLS = tlscfg
	return nil
}

// AfterApply handler for Kong
func (tlsc *TLSConfig) AfterApply() error {
	return tlsc.Load()
}
