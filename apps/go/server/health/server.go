package health

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/tguidoux/opensqs/apps/go/server/serverbase"
)

// Server is a simple health check HTTP server.
// It responds with 200 OK on the /health endpoint.
type Server struct {
	*serverbase.Server
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

	return &Server{
		Server: serverbase.New(port, mux, tlsCfg, 5, 5, 60),
	}
}
