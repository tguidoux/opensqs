package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

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
// A background goroutine periodically removes idle limiters to prevent
// unbounded memory growth. The goroutine is stopped when the returned
// cleanup function is called.
func PerQueueRateLimiter(rps float64, burst int) (Middleware, func()) {
	var (
		mu       sync.Mutex
		limiters = make(map[string]*rate.Limiter)
		stop     = make(chan struct{})
	)

	// Clean up idle limiters every 5 minutes to prevent unbounded growth.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				// Remove limiters that have fully replenished their burst budget,
				// meaning they haven't been used recently.
				for name, l := range limiters {
					if l.Tokens() >= float64(burst) {
						delete(limiters, name)
					}
				}
				mu.Unlock()
			case <-stop:
				return
			}
		}
	}()

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

	mw := func(next http.Handler) http.Handler {
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

	cleanup := func() { close(stop) }
	return mw, cleanup
}

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
