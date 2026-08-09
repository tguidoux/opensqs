package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tguidoux/opensqs/apps/go/server/middleware"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
)

func TestChainOrder(t *testing.T) {
	var order []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw1-after")
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw2-after")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	chained := middleware.Chain(mw1, mw2)(handler)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	chained.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, []string{
		"mw1-before",
		"mw2-before",
		"handler",
		"mw2-after",
		"mw1-after",
	}, order)
}

func TestChainEmpty(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	chained := middleware.Chain()(handler)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	chained.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTeapot, rr.Code)
}

func TestRequestLogger(t *testing.T) {
	log := logger.New("test", logger.UncontextualLoggerType)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	})

	mw := middleware.RequestLogger(log)
	wrapped := mw(handler)

	req := httptest.NewRequest("POST", "/123456789012/my-queue", nil)
	req.Header.Set("User-Agent", "test-agent")
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("X-Request-ID"))
}

func TestRequestLoggerSetsRequestID(t *testing.T) {
	log := logger.New("test", logger.UncontextualLoggerType)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.RequestLogger(log)
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	// X-Request-ID should be set on the response
	requestID := rr.Header().Get("X-Request-ID")
	assert.NotEmpty(t, requestID)
}
