package ui

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
)

// Server is the web dashboard HTTP server.
// It serves server-side rendered HTML pages and JSON API endpoints
// for queue management, mirroring the health server lifecycle pattern.
type Server struct {
	server   *http.Server
	log      logger.LoggerInterface
	tlsCfg   *tls.Config
	certFile string
	keyFile  string
}

// NewServer creates a new UI server on the given port.
// The manager provides direct access to queue operations without
// going through the SQS wire protocol.
// metricsEnabled controls whether the metrics tab is shown.
// If tlsCfg is non-nil, the server will use HTTPS.
func NewServer(port int, manager *queue.QueueManager, log logger.LoggerInterface, metricsEnabled bool, tlsCfg *tls.Config) *Server {
	mux := http.NewServeMux()
	h := newHandler(manager, log, metricsEnabled)

	// HTML pages
	mux.HandleFunc("/", h.handleIndex)
	mux.HandleFunc("/queues/new", h.handleCreateQueueForm)
	mux.HandleFunc("/queues/create", h.handleCreateQueue)
	mux.HandleFunc("/queues/", h.handleQueueRoutes)
	mux.HandleFunc("/metrics", h.handleMetrics)

	// Static assets
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(fmt.Sprintf("failed to create static sub-filesystem: %v", err))
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// JSON API endpoints for auto-refresh
	mux.HandleFunc("/api/queues", h.handleAPIQueues)
	mux.HandleFunc("/api/queues/", h.handleAPIQueueMessages)
	mux.HandleFunc("/api/metrics", h.handleAPIMetrics)

	// Wrap with security headers
	securedMux := withSecurityHeaders(mux)

	s := &Server{
		server: &http.Server{
			Addr:              fmt.Sprintf(":%d", port),
			Handler:           securedMux,
			ReadTimeout:       10 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
		log: log,
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

// Start begins listening for UI requests.
func (s *Server) Start() error {
	if s.tlsCfg != nil {
		return s.server.ListenAndServeTLS(s.certFile, s.keyFile)
	}
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the UI server.
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Handler returns the HTTP handler used by the server.
// This is primarily useful for testing with httptest.
func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

// withSecurityHeaders wraps an http.Handler with standard security headers.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;")
		next.ServeHTTP(w, r)
	})
}
