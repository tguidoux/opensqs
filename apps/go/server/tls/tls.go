package tls

import (
	"crypto/tls"
	"fmt"
)

// Config holds TLS configuration for an HTTP server.
// It mirrors the TLSConfig struct in the server config package
// but is self-contained so this package has no dependency on the config package.
type Config struct {
	// Enabled controls whether TLS is used.
	Enabled bool
	// CertFile is the path to the TLS certificate file (PEM).
	CertFile string
	// KeyFile is the path to the TLS private key file (PEM).
	KeyFile string
}

// LoadTLSConfig loads a TLS certificate and key, returning a *tls.Config
// suitable for use with http.Server.TLSConfig.
// If both certFile and keyFile are empty, TLS is considered disabled
// and the function returns (nil, nil).
func LoadTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	if certFile == "" && keyFile == "" {
		return nil, nil
	}

	if certFile == "" {
		return nil, fmt.Errorf("TLS key file provided but certificate file is missing")
	}
	if keyFile == "" {
		return nil, fmt.Errorf("TLS certificate file provided but key file is missing")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS key pair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// LoadFromConfig loads a TLS config from a Config struct.
// Returns (nil, nil) when TLS is disabled.
func LoadFromConfig(cfg Config) (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	return LoadTLSConfig(cfg.CertFile, cfg.KeyFile)
}
