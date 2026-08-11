package ui_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ui "github.com/tguidoux/opensqs/apps/go/server/ui"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/credentials"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
)

func newTestManager() *queue.QueueManager {
	factory := func(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) (store.Store, error) {
		return memory.NewMemoryStore(queueName, visibilityTimeout, serverSecret, cfg), nil
	}
	return queue.NewQueueManager("localhost:9324", "000000000000", "us-east-1", []byte("test-secret"), factory)
}

func newTestCredStore() credentials.CredentialStore {
	return credentials.NewMemoryCredentialStore()
}

func newTestServer() *ui.Server {
	log := logger.New("ui-test", logger.UncontextualLoggerType)
	return ui.NewServer(0, newTestManager(), newTestCredStore(), log, false, nil)
}

func newTestServerWithMetrics() *ui.Server {
	log := logger.New("ui-test", logger.UncontextualLoggerType)
	return ui.NewServer(0, newTestManager(), newTestCredStore(), log, true, nil)
}

func TestIndexPageEmptyQueues(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "No queues found")
}

func TestIndexPageWithQueue(t *testing.T) {
	srv := newTestServer()
	// Create a queue via the manager
	manager := newTestManager()
	log := logger.New("ui-test", logger.UncontextualLoggerType)
	srv = ui.NewServer(0, manager, newTestCredStore(), log, false, nil)

	_, err := manager.CreateQueue("test-queue", nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test-queue")
	assert.Contains(t, w.Body.String(), "Standard")
}

func TestCreateQueueForm(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/queues/new", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Create Queue")
	assert.Contains(t, w.Body.String(), "queueName")
}

func TestCreateQueuePOST(t *testing.T) {
	srv := newTestServer()
	form := "queueName=my-new-queue"
	req := httptest.NewRequest(http.MethodPost, "/queues/create", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/queues/my-new-queue")
}

func TestCreateFifoQueuePOST(t *testing.T) {
	srv := newTestServer()
	form := "queueName=my-fifo-queue.fifo&fifoQueue=on"
	req := httptest.NewRequest(http.MethodPost, "/queues/create", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/queues/my-fifo-queue.fifo")
}

func TestCreateFifoQueueMissingSuffix(t *testing.T) {
	srv := newTestServer()
	form := "queueName=bad-fifo&fifoQueue=on"
	req := httptest.NewRequest(http.MethodPost, "/queues/create", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/queues/new?error=")
}

func TestQueueDetailPage(t *testing.T) {
	manager := newTestManager()
	log := logger.New("ui-test", logger.UncontextualLoggerType)
	srv := ui.NewServer(0, manager, newTestCredStore(), log, false, nil)

	_, err := manager.CreateQueue("detail-queue", nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/queues/detail-queue", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "detail-queue")
	assert.Contains(t, w.Body.String(), "Attributes")
	assert.Contains(t, w.Body.String(), "Send Message")
}

func TestQueueDetailNotFound(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/queues/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
}

func TestDeleteQueuePOST(t *testing.T) {
	manager := newTestManager()
	log := logger.New("ui-test", logger.UncontextualLoggerType)
	srv := ui.NewServer(0, manager, newTestCredStore(), log, false, nil)

	_, err := manager.CreateQueue("to-delete", nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/queues/to-delete/delete", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/?success=Queue+deleted")

	// Verify queue is gone
	_, err = manager.LookupQueue("to-delete")
	assert.Error(t, err)
}

func TestPurgeQueuePOST(t *testing.T) {
	manager := newTestManager()
	log := logger.New("ui-test", logger.UncontextualLoggerType)
	srv := ui.NewServer(0, manager, newTestCredStore(), log, false, nil)

	_, err := manager.CreateQueue("to-purge", nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/queues/to-purge/purge", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/queues/to-purge?success=Queue+purged")
}

func TestAPIQueuesEmpty(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/queues", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var items []map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &items)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestAPIQueuesWithData(t *testing.T) {
	manager := newTestManager()
	log := logger.New("ui-test", logger.UncontextualLoggerType)
	srv := ui.NewServer(0, manager, newTestCredStore(), log, false, nil)

	_, err := manager.CreateQueue("api-queue", nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/queues", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var items []map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &items)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "api-queue", items[0]["Name"])
}

func TestAPIQueueMessages(t *testing.T) {
	manager := newTestManager()
	log := logger.New("ui-test", logger.UncontextualLoggerType)
	srv := ui.NewServer(0, manager, newTestCredStore(), log, false, nil)

	_, err := manager.CreateQueue("msg-queue", nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/queues/msg-queue/messages", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var msgs []map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &msgs)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestStaticCSS(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "--bg")
}

func TestStaticJS(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "theme-toggle")
}

func TestIndexPageNotFound(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/nonexistent-page", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMetricsPageDisabled(t *testing.T) {
	srv := newTestServer() // metricsEnabled = false
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMetricsPageEnabled(t *testing.T) {
	srv := newTestServerWithMetrics()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Metrics")
	assert.Contains(t, w.Body.String(), "API Requests")
	assert.Contains(t, w.Body.String(), "Queue Sizes")
}

func TestMetricsPageHasNavLink(t *testing.T) {
	srv := newTestServerWithMetrics()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `href="/metrics"`)
	assert.Contains(t, w.Body.String(), "Metrics")
}

func TestMetricsPageNoNavLinkWhenDisabled(t *testing.T) {
	srv := newTestServer() // metricsEnabled = false
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), `href="/metrics"`)
}

func TestAPIMetricsDisabled(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAPIMetricsEnabled(t *testing.T) {
	srv := newTestServerWithMetrics()
	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var data map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &data)
	require.NoError(t, err)
	// Should have metrics fields
	assert.Contains(t, data, "APIRequests")
	assert.Contains(t, data, "QueueSizes")
}

// --- Credential tests ---

func TestCredentialsPageEmpty(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/credentials", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "No credentials found")
}

func TestCredentialsPageHasNavLink(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `href="/credentials"`)
	assert.Contains(t, w.Body.String(), "Credentials")
}

func TestCreateCredentialPOST(t *testing.T) {
	srv := newTestServer()
	form := "label=test-cred&region=us-west-2&accountId=999999999999"
	req := httptest.NewRequest(http.MethodPost, "/credentials/create", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Credential Created")
	assert.Contains(t, body, "test-cred")
	assert.Contains(t, body, "us-west-2")
	assert.Contains(t, body, "999999999999")
	assert.Contains(t, body, "AKIA")
	assert.Contains(t, body, "export AWS_ACCESS_KEY_ID")
	assert.Contains(t, body, "export AWS_SECRET_ACCESS_KEY")
}

func TestCreateCredentialDefaultsRegionAndAccount(t *testing.T) {
	srv := newTestServer()
	form := "label=defaults-test"
	req := httptest.NewRequest(http.MethodPost, "/credentials/create", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// Should default to the manager's region and account ID
	assert.Contains(t, body, "us-east-1")
	assert.Contains(t, body, "000000000000")
}

func TestDeleteCredentialPOST(t *testing.T) {
	credStore := newTestCredStore()
	manager := newTestManager()
	log := logger.New("ui-test", logger.UncontextualLoggerType)
	srv := ui.NewServer(0, manager, credStore, log, false, nil)

	// Create a credential first
	cred, err := credStore.Create("to-delete", "us-east-1", "123456789012")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/credentials/"+cred.ID+"/delete", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/credentials?success=Credential+deleted")

	// Verify credential is gone
	_, err = credStore.Get(cred.ID)
	assert.Error(t, err)
}

func TestAPICredentialsEmpty(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/credentials", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var items []map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &items)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestAPICredentialsWithData(t *testing.T) {
	credStore := newTestCredStore()
	manager := newTestManager()
	log := logger.New("ui-test", logger.UncontextualLoggerType)
	srv := ui.NewServer(0, manager, credStore, log, false, nil)

	_, err := credStore.Create("api-test", "eu-west-1", "555555555555")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/credentials", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var items []map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &items)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "api-test", items[0]["Label"])
	assert.Equal(t, "eu-west-1", items[0]["Region"])
	// Secret should NOT be in the API response (empty string, not the actual secret)
	secret, _ := items[0]["SecretAccessKey"].(string)
	assert.Empty(t, secret)
}
