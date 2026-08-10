package ui

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
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

	// Wrap with security headers and CSRF protection
	securedMux := withCSRFProtection(withSecurityHeaders(mux))

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
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// HSTS only makes sense over HTTPS — only set it when the request
		// is already over TLS (directly or via a TLS-terminating proxy).
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// withCSRFProtection rejects cross-origin POST/PUT/DELETE/PATCH requests
// by validating the Origin header against the request host.
// This is a lightweight CSRF defense that does not require session state.
// Same-origin browsers always send an Origin or Referer header on cross-site
// form submissions; if neither is present, the request is allowed only for
// safe methods (GET, HEAD, OPTIONS).
func withCSRFProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only check state-changing methods
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			// Fall back to Referer header
			referer := r.Header.Get("Referer")
			if referer == "" {
				// No Origin or Referer — reject for safety
				http.Error(w, "Forbidden: missing Origin header", http.StatusForbidden)
				return
			}
			origin = referer
		}

		// Parse the origin to extract the host
		// Origin can be "https://host:port" or "host:port"
		originHost := origin
		if strings.HasPrefix(origin, "http://") {
			originHost = strings.TrimPrefix(origin, "http://")
		} else if strings.HasPrefix(origin, "https://") {
			originHost = strings.TrimPrefix(origin, "https://")
		}
		// Strip path from Referer
		if idx := strings.Index(originHost, "/"); idx >= 0 {
			originHost = originHost[:idx]
		}

		requestHost := r.Host
		if originHost != requestHost {
			http.Error(w, "Forbidden: cross-origin request rejected", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
