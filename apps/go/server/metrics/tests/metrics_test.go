package metrics_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tguidoux/opensqs/apps/go/server/metrics"
)

// newTestCollector creates a Collector with a private registry for testing.
func newTestCollector() (*metrics.Collector, *prometheus.Registry) {
	reg := prometheus.NewRegistry()
	c := metrics.NewCollectorWithRegistry(reg)
	return c, reg
}

// scrapeMetrics gathers all metrics from the registry and returns them as text.
func scrapeMetrics(t *testing.T, reg *prometheus.Registry) string {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)

	var sb strings.Builder
	for _, mf := range mfs {
		// Write metric family name
		sb.WriteString(mf.GetName())
		sb.WriteString("\n")
		for _, m := range mf.GetMetric() {
			// Write labels
			labels := make([]string, 0, len(m.GetLabel()))
			for _, lp := range m.GetLabel() {
				labels = append(labels, lp.GetName()+"=\""+lp.GetValue()+"\"")
			}
			labelStr := ""
			if len(labels) > 0 {
				labelStr = "{" + strings.Join(labels, ",") + "}"
			}

			if m.GetCounter() != nil {
				sb.WriteString(mf.GetName())
				sb.WriteString(labelStr)
				sb.WriteString(" ")
				sb.WriteString(formatFloat(m.GetCounter().GetValue()))
				sb.WriteString("\n")
			}
			if m.GetGauge() != nil {
				sb.WriteString(mf.GetName())
				sb.WriteString(labelStr)
				sb.WriteString(" ")
				sb.WriteString(formatFloat(m.GetGauge().GetValue()))
				sb.WriteString("\n")
			}
			if m.GetHistogram() != nil {
				sb.WriteString(mf.GetName())
				sb.WriteString("_count")
				sb.WriteString(labelStr)
				sb.WriteString(" ")
				sb.WriteString(formatFloat(float64(m.GetHistogram().GetSampleCount())))
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

func formatFloat(f float64) string {
	// Simple formatting: if whole number, show as integer
	if f == float64(int64(f)) {
		return formatInt(int64(f))
	}
	// Fallback for non-whole numbers
	return "0"
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// metricContains checks if a specific metric+labels combination exists in the text output.
func metricContains(text, metricName string, labels map[string]string) bool {
	// Build expected label string
	labelParts := make([]string, 0, len(labels))
	for k, v := range labels {
		labelParts = append(labelParts, k+"=\""+v+"\"")
	}
	expected := metricName + "{" + strings.Join(labelParts, ",") + "}"
	return strings.Contains(text, expected)
}

// --- Collector Tests ---

func TestCollector_MessagesSent(t *testing.T) {
	c, reg := newTestCollector()

	c.IncMessagesSent("my-queue")
	c.IncMessagesSent("my-queue")
	c.IncMessagesSent("other-queue")

	text := scrapeMetrics(t, reg)
	assert.Contains(t, text, `opensqs_messages_sent_total{queue="my-queue"} 2`)
	assert.Contains(t, text, `opensqs_messages_sent_total{queue="other-queue"} 1`)
}

func TestCollector_MessagesReceived(t *testing.T) {
	c, reg := newTestCollector()

	c.IncMessagesReceived("my-queue", 5)
	c.IncMessagesReceived("my-queue", 3)

	text := scrapeMetrics(t, reg)
	assert.Contains(t, text, `opensqs_messages_received_total{queue="my-queue"} 8`)
}

func TestCollector_MessagesDeleted(t *testing.T) {
	c, reg := newTestCollector()

	c.IncMessagesDeleted("my-queue")
	c.IncMessagesDeleted("my-queue")
	c.IncMessagesDeleted("my-queue")

	text := scrapeMetrics(t, reg)
	assert.Contains(t, text, `opensqs_messages_deleted_total{queue="my-queue"} 3`)
}

func TestCollector_QueueSize(t *testing.T) {
	c, reg := newTestCollector()

	c.SetQueueSize("my-queue", "available", 42)
	c.SetQueueSize("my-queue", "inflight", 3)
	c.SetQueueSize("my-queue", "delayed", 7)

	text := scrapeMetrics(t, reg)
	assert.Contains(t, text, `opensqs_queue_size{queue="my-queue",type="available"} 42`)
	assert.Contains(t, text, `opensqs_queue_size{queue="my-queue",type="inflight"} 3`)
	assert.Contains(t, text, `opensqs_queue_size{queue="my-queue",type="delayed"} 7`)
}

func TestCollector_APIRequest(t *testing.T) {
	c, reg := newTestCollector()

	c.IncAPIRequest("SendMessage", "query")
	c.IncAPIRequest("SendMessage", "query")
	c.IncAPIRequest("ReceiveMessage", "json")

	text := scrapeMetrics(t, reg)
	assert.Contains(t, text, `opensqs_api_requests_total{action="SendMessage",protocol="query"} 2`)
	assert.Contains(t, text, `opensqs_api_requests_total{action="ReceiveMessage",protocol="json"} 1`)
}

func TestCollector_APIRequestDuration(t *testing.T) {
	c, reg := newTestCollector()

	c.ObserveAPIRequestDuration("SendMessage", "query", 0.05)
	c.ObserveAPIRequestDuration("SendMessage", "query", 0.15)

	text := scrapeMetrics(t, reg)
	// Histogram should have 2 samples
	assert.Contains(t, text, `opensqs_api_request_duration_seconds_count{action="SendMessage",protocol="query"} 2`)
}

func TestCollector_MoveTaskMetrics(t *testing.T) {
	c, reg := newTestCollector()

	srcArn := "arn:aws:sqs:us-east-1:123:src"
	dstArn := "arn:aws:sqs:us-east-1:123:dst"

	c.IncMoveTaskMessagesMoved(srcArn, dstArn)
	c.IncMoveTaskMessagesMoved(srcArn, dstArn)
	c.IncMoveTaskMessagesMoved(srcArn, dstArn)

	text := scrapeMetrics(t, reg)
	assert.Contains(t, text, `opensqs_move_task_messages_moved_total{destination_arn="`+dstArn+`",source_arn="`+srcArn+`"} 3`)

	c.IncMoveTaskActive()
	c.IncMoveTaskActive()

	text = scrapeMetrics(t, reg)
	assert.Contains(t, text, `opensqs_move_task_active 2`)

	c.DecMoveTaskActive()
	text = scrapeMetrics(t, reg)
	assert.Contains(t, text, `opensqs_move_task_active 1`)
}

// --- Server Tests ---

func TestServer_MetricsEndpoint(t *testing.T) {
	reg := prometheus.NewRegistry()
	collector := metrics.NewCollectorWithRegistry(reg)

	// Record some metrics
	collector.IncMessagesSent("test-queue")
	collector.IncAPIRequest("SendMessage", "query")

	// Create a test server with the custom registry
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	bodyStr := string(body)
	assert.Contains(t, bodyStr, "opensqs_messages_sent_total")
	assert.Contains(t, bodyStr, "opensqs_api_requests_total")
	assert.Contains(t, bodyStr, "test-queue")
	assert.Contains(t, bodyStr, "SendMessage")
}

func TestServer_GracefulShutdown(t *testing.T) {
	srv := metrics.NewServer(19328, nil)

	go func() {
		_ = srv.Start()
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Verify it responds
	resp, err := http.Get("http://localhost:19328/metrics")
	if err == nil {
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = srv.Stop(ctx)
	assert.NoError(t, err)
}

// TestCollector_AllMetricsRegistered verifies that all expected metric names
// are present when the collector is created and metrics are initialized.
func TestCollector_AllMetricsRegistered(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := metrics.NewCollectorWithRegistry(reg)

	// Initialize all metrics by using them at least once (Prometheus lazily
	// creates metric instances on first use)
	c.IncMessagesSent("init")
	c.IncMessagesReceived("init", 1)
	c.IncMessagesDeleted("init")
	c.SetQueueSize("init", "available", 0)
	c.IncAPIRequest("init", "init")
	c.ObserveAPIRequestDuration("init", "init", 0)
	c.IncMoveTaskMessagesMoved("init", "init")
	c.IncMoveTaskActive()
	c.DecMoveTaskActive()

	mfs, err := reg.Gather()
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}

	expectedMetrics := []string{
		"opensqs_messages_sent_total",
		"opensqs_messages_received_total",
		"opensqs_messages_deleted_total",
		"opensqs_queue_size",
		"opensqs_api_requests_total",
		"opensqs_api_request_duration_seconds",
		"opensqs_move_task_messages_moved_total",
		"opensqs_move_task_active",
	}

	for _, name := range expectedMetrics {
		assert.True(t, names[name], "expected metric %s to be registered", name)
	}
}

// TestServer_MetricsContentType verifies the /metrics endpoint returns
// the correct content type.
func TestServer_MetricsContentType(t *testing.T) {
	reg := prometheus.NewRegistry()
	_ = metrics.NewCollectorWithRegistry(reg)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	assert.True(t,
		strings.HasPrefix(ct, "text/plain"),
		"expected text/plain content type, got: %s", ct)
}
