package middleware

import "net/http"

// Middleware wraps an http.Handler with additional behavior.
// It follows the standard Go middleware pattern: func(http.Handler) http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain composes multiple middlewares into a single middleware.
// Middlewares are applied in order: the first middleware in the list
// is the outermost (executed first on incoming requests).
//
// Example: Chain(RequestLogger(log), RateLimiter(rps), handler)
// Request → RequestLogger → RateLimiter → handler
func Chain(middlewares ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		h := final
		// Apply in reverse so the first middleware in the list is outermost.
		for i := len(middlewares) - 1; i >= 0; i-- {
			h = middlewares[i](h)
		}
		return h
	}
}
