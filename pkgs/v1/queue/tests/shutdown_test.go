package queue_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/sqlite"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func TestShutdown_NoQueues(t *testing.T) {
	qm := newTestManager(t)
	err := qm.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestShutdown_ClosesAllStores(t *testing.T) {
	qm := newTestManager(t)

	// Create multiple queues
	for _, name := range []string{"q1", "q2", "q3"} {
		_, err := qm.CreateQueue(name, nil)
		require.NoError(t, err)
	}

	// Send messages to each
	for _, name := range []string{"q1", "q2", "q3"} {
		q, err := qm.LookupQueue(name)
		require.NoError(t, err)
		err = q.Store().SendMessage(context.Background(), &types.Message{
			MessageID: "msg-1",
			Body:      "hello",
			IsVisible: true,
		}, 0)
		require.NoError(t, err)
	}

	// Shutdown
	err := qm.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestShutdown_Idempotent(t *testing.T) {
	qm := newTestManager(t)

	_, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)

	// First shutdown
	err = qm.Shutdown(context.Background())
	assert.NoError(t, err)

	// Second shutdown should also succeed (Close is idempotent for memory store)
	err = qm.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestShutdown_WithDeadline(t *testing.T) {
	qm := newTestManager(t)

	_, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)

	// Shutdown with a short deadline — should still succeed for memory store
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = qm.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestShutdown_AfterOperations(t *testing.T) {
	qm := newTestManager(t)

	q, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)

	// Perform various operations
	_ = q.Store().SendMessage(context.Background(), &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
		IsVisible: true,
	}, 0)

	msgs, err := q.Store().ReceiveMessages(context.Background(), 10, 30, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	_ = q.Store().DeleteMessage(context.Background(), msgs[0].ReceiptHandle)

	// Shutdown after operations
	err = qm.Shutdown(context.Background())
	assert.NoError(t, err)
}

// TestShutdown_WithSQLiteStore verifies shutdown works with SQLite stores.
func TestShutdown_WithSQLiteStore(t *testing.T) {
	factory := func(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) (store.Store, error) {
		return newSQLiteStore(queueName, visibilityTimeout, serverSecret, cfg)
	}

	qm := queue.NewQueueManager("localhost:9324", "000000000000", "us-east-1", []byte("test-secret"), factory, logger.New("test", logger.UncontextualLoggerType))

	_, err := qm.CreateQueue("sqlite-q", nil)
	require.NoError(t, err)

	err = qm.Shutdown(context.Background())
	assert.NoError(t, err)
}

// newSQLiteStore creates a SQLite store for testing using a temporary file.
func newSQLiteStore(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) (store.Store, error) {
	dbPath := filepath.Join(os.TempDir(), "opensqs-shutdown-test-"+queueName+".db")
	os.Remove(dbPath) // Clean up any stale file
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	return sqlite.NewSQLiteStore(db, queueName, visibilityTimeout, serverSecret, cfg)
}
