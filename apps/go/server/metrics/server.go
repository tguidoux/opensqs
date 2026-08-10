package metrics

import (
	"crypto/tls"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tguidoux/opensqs/apps/go/server/serverbase"
)

// Server is the Prometheus metrics HTTP server.
// It serves the /metrics endpoint for Prometheus scraping.
type Server struct {
	*serverbase.Server
}

// NewServer creates a new metrics server on the given port.
// If tlsCfg is non-nil, the server will use HTTPS.
func NewServer(port int, tlsCfg *tls.Config) *Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return &Server{
		Server: serverbase.New(port, mux, tlsCfg, 10, 10, 60),
	}
}
