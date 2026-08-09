package middleware

import (
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// GlobalRateLimiter creates a middleware that limits the total request rate
// across all queues using a single token bucket.
// When the rate is exceeded, it responds with 429 Too Many Requests.
func GlobalRateLimiter(rps float64, burst int) Middleware {
	limiter := rate.NewLimiter(rate.Limit(rps), burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// PerQueueRateLimiter creates a middleware that limits the request rate
// per queue. The queue name is extracted from the URL path
// (/{accountId}/{queueName}). Each queue gets its own token bucket.
// When the rate is exceeded, it responds with 429 Too Many Requests.
func PerQueueRateLimiter(rps float64, burst int) Middleware {
	var (
		mu       sync.Mutex
		limiters = make(map[string]*rate.Limiter)
	)

	getLimiter := func(queueName string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		l, exists := limiters[queueName]
		if !exists {
			l = rate.NewLimiter(rate.Limit(rps), burst)
			limiters[queueName] = l
		}
		return l
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			queueName := ExtractQueueName(r.URL.Path)
			if queueName == "" {
				// No queue in path — allow through (e.g. health checks, list queues)
				next.ServeHTTP(w, r)
				return
			}

			limiter := getLimiter(queueName)
			if !limiter.Allow() {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractQueueName parses the queue name from the URL path.
// SQS queue URLs follow the format: /{accountId}/{queueName}
// Returns an empty string if no queue name is found.
// ExtractQueueName parses the queue name from an SQS URL path.
// SQS paths follow the pattern /{accountId}/{queueName}.
// Returns empty string if the path doesn't match.
func ExtractQueueName(path string) string {
	// Strip query string
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}

	// Strip leading slash
	path = strings.TrimPrefix(path, "/")

	// Split into segments
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return ""
	}

	// parts[0] = accountId, parts[1] = queueName
	queueName := parts[1]
	if queueName == "" {
		return ""
	}

	return queueName
}
