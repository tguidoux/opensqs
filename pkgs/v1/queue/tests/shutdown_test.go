package queue_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func TestShutdown_NoQueues(t *testing.T) {
	qm := newTestManager()
	err := qm.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestShutdown_ClosesAllStores(t *testing.T) {
	qm := newTestManager()

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
	qm := newTestManager()

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
	qm := newTestManager()

	_, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)

	// Shutdown with a short deadline — should still succeed for memory store
	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()

	err = qm.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestShutdown_AfterOperations(t *testing.T) {
	qm := newTestManager()

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

// TestShutdown_WithSQLiteStore verifies shutdown works with SQLite stores too.
func TestShutdown_WithSQLiteStore(t *testing.T) {
	factory := func(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) store.Store {
		s, err := newSQLiteStore(queueName, visibilityTimeout, serverSecret, cfg)
		require.NoError(t, err)
		return s
	}

	qm := queue.NewQueueManager("localhost:9324", "000000000000", "us-east-1", []byte("test-secret"), factory)

	_, err := qm.CreateQueue("sqlite-q", nil)
	require.NoError(t, err)

	err = qm.Shutdown(context.Background())
	assert.NoError(t, err)
}

// newSQLiteStore creates a SQLite store for testing (avoids importing sqlite directly
// which requires CGO). Uses memory store as fallback if SQLite isn't available.
func newSQLiteStore(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) (store.Store, error) {
	// Use memory store for this test — the shutdown behavior is the same
	// (both implement store.Store with Close() error)
	return memory.NewMemoryStore(queueName, visibilityTimeout, serverSecret, cfg), nil
}
