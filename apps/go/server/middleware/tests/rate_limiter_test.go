package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tguidoux/opensqs/apps/go/server/middleware"
)

func TestGlobalRateLimiterAllows(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.GlobalRateLimiter(100, 10)
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGlobalRateLimiterRejects(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Very low rate: 1 req/sec, burst of 1
	mw := middleware.GlobalRateLimiter(1, 1)
	wrapped := mw(handler)

	// First request should pass (uses the burst token)
	req1 := httptest.NewRequest("GET", "/", nil)
	rr1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Second request immediately should be rejected
	req2 := httptest.NewRequest("GET", "/", nil)
	rr2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
	assert.NotEmpty(t, rr2.Header().Get("Retry-After"))
}

func TestPerQueueRateLimiterAllows(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.PerQueueRateLimiter(100, 10)
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/123456789012/my-queue", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestPerQueueRateLimiterRejects(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.PerQueueRateLimiter(1, 1)
	wrapped := mw(handler)

	// First request for queue-a should pass
	req1 := httptest.NewRequest("GET", "/123456789012/queue-a", nil)
	rr1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Second request for queue-a immediately should be rejected
	req2 := httptest.NewRequest("GET", "/123456789012/queue-a", nil)
	rr2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)

	// First request for queue-b should pass (separate limiter)
	req3 := httptest.NewRequest("GET", "/123456789012/queue-b", nil)
	rr3 := httptest.NewRecorder()
	wrapped.ServeHTTP(rr3, req3)
	assert.Equal(t, http.StatusOK, rr3.Code)
}

func TestPerQueueRateLimiterRefills(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.PerQueueRateLimiter(100, 1)
	wrapped := mw(handler)

	// First request passes
	req1 := httptest.NewRequest("GET", "/123456789012/q", nil)
	rr1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Wait for token refill
	time.Sleep(20 * time.Millisecond)

	// Second request should pass after refill
	req2 := httptest.NewRequest("GET", "/123456789012/q", nil)
	rr2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)
}

func TestExtractQueueName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/123456789012/my-queue", "my-queue"},
		{"/123456789012/my-queue?Action=SendMessage", "my-queue"},
		{"/000000000000/test.fifo", "test.fifo"},
		{"/", ""},
		{"/onlyone", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := middleware.ExtractQueueName(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}
