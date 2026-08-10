package middleware

import (
	"net/http"
	"time"

	"github.com/tguidoux/opensqs/pkgs/v1/id"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
)

// RequestLogger creates a middleware that logs each HTTP request with structured fields.
// It generates a unique request ID, records the start time, captures the response
// status code and size, and logs the request after it completes.
func RequestLogger(log logger.LoggerInterface) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := generateRequestID()
			start := time.Now()

			// Add request ID to response headers for client-side correlation
			w.Header().Set("X-Request-ID", requestID)

			sw := newStatusResponseWriter(w)
			next.ServeHTTP(sw, r)

			duration := time.Since(start)

			log.Info("http request", map[string]any{
				"requestId":  requestID,
				"method":     r.Method,
				"path":       r.URL.Path,
				"status":     sw.statusCode,
				"bytesOut":   sw.bytesWritten,
				"durationMs": duration.Milliseconds(),
				"remoteAddr": r.RemoteAddr,
				"userAgent":  r.UserAgent(),
			})
		})
	}
}

// generateRequestID creates a unique request ID.
// Uses crypto/rand for uniqueness via the shared id package.
func generateRequestID() string {
	return id.NewHexID()
}
