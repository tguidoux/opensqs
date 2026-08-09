package metrics

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server is the Prometheus metrics HTTP server.
// It serves the /metrics endpoint for Prometheus scraping.
type Server struct {
	server   *http.Server
	tlsCfg   *tls.Config
	certFile string
	keyFile  string
}

// NewServer creates a new metrics server on the given port.
// If tlsCfg is non-nil, the server will use HTTPS.
func NewServer(port int, tlsCfg *tls.Config) *Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	s := &Server{
		server: &http.Server{
			Addr:              fmt.Sprintf(":%d", port),
			Handler:           mux,
			ReadTimeout:       10 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
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

// Start begins listening for metrics scrape requests.
func (s *Server) Start() error {
	if s.tlsCfg != nil {
		return s.server.ListenAndServeTLS(s.certFile, s.keyFile)
	}
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the metrics server.
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
