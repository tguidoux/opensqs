package serverbase

// Package serverbase provides a shared HTTP server lifecycle pattern
// used by the health, metrics, and UI servers to eliminate duplication.

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

// Server provides the common HTTP server lifecycle: Start, Stop, SetCertFiles.
// Servers in health, metrics, and ui embed this struct and add their own
// domain-specific fields.
type Server struct {
	server   *http.Server
	tlsCfg   *tls.Config
	certFile string
	keyFile  string
}

// New creates a base Server with standard timeouts.
// The caller provides the handler, port, TLS config, and timeouts.
func New(port int, handler http.Handler, tlsCfg *tls.Config, readTimeout, writeTimeout, idleTimeout time.Duration) *Server {
	s := &Server{
		server: &http.Server{
			Addr:              fmt.Sprintf(":%d", port),
			Handler:           handler,
			ReadTimeout:       readTimeout,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
		},
	}

	if tlsCfg != nil {
		s.server.TLSConfig = tlsCfg
		s.tlsCfg = tlsCfg
	}

	return s
}

// SetCertFiles sets the certificate and key file paths for TLS.
// Must be called before Start() if TLS is enabled.
func (s *Server) SetCertFiles(certFile, keyFile string) {
	s.certFile = certFile
	s.keyFile = keyFile
}

// Start begins listening for requests.
func (s *Server) Start() error {
	if s.tlsCfg != nil {
		return s.server.ListenAndServeTLS(s.certFile, s.keyFile)
	}
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Handler returns the HTTP handler used by the server.
// This is primarily useful for testing with httptest.
func (s *Server) Handler() http.Handler {
	return s.server.Handler
}
