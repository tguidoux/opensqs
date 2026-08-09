package health

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

// Server is a simple health check HTTP server.
// It responds with 200 OK on the /health endpoint.
type Server struct {
	server   *http.Server
	tlsCfg   *tls.Config
	certFile string
	keyFile  string
}

// NewServer creates a new health check server on the given port.
// If tlsCfg is non-nil, the server will use HTTPS.
func NewServer(port int, tlsCfg *tls.Config) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy"}`)
	})

	s := &Server{
		server: &http.Server{
			Addr:              fmt.Sprintf(":%d", port),
			Handler:           mux,
			ReadTimeout:       5 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      5 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}

	if tlsCfg != nil {
		s.server.TLSConfig = tlsCfg
		s.tlsCfg = tlsCfg
		s.certFile = "" // will be set via SetCertFiles
	}

	return s
}

// SetCertFiles sets the certificate and key file paths for TLS.
// Must be called before Start() if TLS is enabled.
func (s *Server) SetCertFiles(certFile, keyFile string) {
	s.certFile = certFile
	s.keyFile = keyFile
}

// Start begins listening for health check requests.
func (s *Server) Start() error {
	if s.tlsCfg != nil {
		return s.server.ListenAndServeTLS(s.certFile, s.keyFile)
	}
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the health check server.
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
