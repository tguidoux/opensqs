package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/apps/go/server/handlers"
	"github.com/tguidoux/opensqs/apps/go/server/middleware"
	"github.com/tguidoux/opensqs/apps/go/server/protocol"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
)

// testServer is a lightweight HTTP test server wrapping the SQS handler.
// It avoids the full main.go startup so tests can run in isolation.
type testServer struct {
	handler http.Handler
	server  *http.Server
	baseURL string
	manager *queue.QueueManager
}

// newTestServer creates and starts a test HTTP server on a random port.
// It uses memory storage and returns the base URL for making requests.
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	factory := func(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) store.Store {
		return memory.NewMemoryStore(queueName, visibilityTimeout, serverSecret, cfg)
	}
	manager := queue.NewQueueManager("localhost:0", "123456789012", "us-east-1", []byte("test-secret"), factory)
	limits := queue.NewLimits(queue.StrictMode)
	handler := handlers.NewHandler(manager, limits, true, nil) // autoCreate=true

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleTestRequest(w, r, handler)
	})

	// Wrap with request logging middleware for realistic testing
	log := logger.New("integration-test", logger.UncontextualLoggerType)
	wrappedHandler := middleware.Chain(
		middleware.RequestLogger(log),
	)(mux)

	server := &http.Server{
		Handler:      wrappedHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server.Addr = listener.Addr().String()
	go func() {
		_ = server.Serve(listener)
	}()

	ts := &testServer{
		handler: wrappedHandler,
		server:  server,
		baseURL: fmt.Sprintf("http://%s", listener.Addr().String()),
		manager: manager,
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})

	return ts
}

// handleTestRequest processes an incoming SQS API request for integration tests.
func handleTestRequest(w http.ResponseWriter, r *http.Request, handler *handlers.Handler) {
	// Parse as Query Protocol (form-urlencoded)
	var queryStr string
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		queryStr = string(body)
	} else {
		queryStr = r.URL.RawQuery
	}

	// Use the real protocol parser instead of a mock
	queryReq, err := protocol.ParseQueryRequest(queryStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := &handlers.QueryRequestAdapter{QueryRequest: queryReq}

	resp, err := handler.HandleRequest(r.Context(), req, handlers.QueryProtocol)
	if err != nil {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusBadRequest)
		// Write a simple error response
		fmt.Fprintf(w, `<?xml version="1.0"?><ErrorResponse><Error><Code>%s</Code><Message>%s</Message></Error></ErrorResponse>`, "Error", err.Error())
		return
	}

	data, err := handlers.MarshalResponse(resp, handlers.QueryProtocol)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// makeQueryRequest sends a POST request with form-urlencoded body.
func (ts *testServer) post(action string, params ...string) (*http.Response, string, error) {
	form := url.Values{}
	form.Set("Action", action)
	for i := 0; i+1 < len(params); i += 2 {
		form.Set(params[i], params[i+1])
	}

	resp, err := http.Post(ts.baseURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return resp, string(body), err
}

// extractXMLValue extracts the text content of an XML element.
func extractXMLValue(xml, tagName string) string {
	openTag := "<" + tagName + ">"
	closeTag := "</" + tagName + ">"
	start := strings.Index(xml, openTag)
	if start == -1 {
		return ""
	}
	start += len(openTag)
	end := strings.Index(xml[start:], closeTag)
	if end == -1 {
		return ""
	}
	return xml[start : start+end]
}

// extractAllXMLValues extracts all occurrences of an XML element.
func extractAllXMLValues(xml, tagName string) []string {
	var results []string
	openTag := "<" + tagName + ">"
	closeTag := "</" + tagName + ">"
	searchFrom := 0
	for {
		start := strings.Index(xml[searchFrom:], openTag)
		if start == -1 {
			break
		}
		start += searchFrom + len(openTag)
		end := strings.Index(xml[start:], closeTag)
		if end == -1 {
			break
		}
		results = append(results, xml[start:start+end])
		searchFrom = start + end + len(closeTag)
	}
	return results
}
