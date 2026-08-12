package sqlite_test

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/sqlite"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func newTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dbPath := "/tmp/opensqs-test-" + generateID() + ".db"
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}
	return db, cleanup
}

func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func newTestStore(t *testing.T) (*sqlite.SQLiteStore, func()) {
	t.Helper()
	db, cleanup := newTestDB(t)
	s, err := sqlite.NewSQLiteStore(db, "test-queue", 30, []byte("test-secret"), store.StoreConfig{})
	require.NoError(t, err)
	return s, cleanup
}

// ---------------------------------------------------------------------------
// Basic Store Operations
// ---------------------------------------------------------------------------

func TestSQLite_SendMessage(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
	}

	err := s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	assert.Equal(t, 1, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesDelayed())
}

func TestSQLite_SendMessageDelayed(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	msg := &types.Message{MessageID: "msg-1", Body: "hello"}
	err := s.SendMessage(context.Background(), msg, 60)
	require.NoError(t, err)

	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())
	assert.Equal(t, 1, s.ApproximateNumberOfMessagesDelayed())
}

func TestSQLite_ReceiveMessage(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	msg := &types.Message{MessageID: "msg-1", Body: "hello"}
	require.NoError(t, s.SendMessage(context.Background(), msg, 0))

	result, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "msg-1", result[0].MessageID)
	assert.NotEmpty(t, result[0].ReceiptHandle)
	assert.Equal(t, 1, result[0].ApproximateReceiveCount)

	// Message should be in-flight
	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
	assert.Equal(t, 1, s.ApproximateNumberOfMessagesNotVisible())
}

func TestSQLite_ReceiveMessageEmpty(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	result, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestSQLite_DeleteMessage(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	msg := &types.Message{MessageID: "msg-1", Body: "hello"}
	require.NoError(t, s.SendMessage(context.Background(), msg, 0))

	result, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	err = s.DeleteMessage(context.Background(), result[0].ReceiptHandle)
	require.NoError(t, err)

	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())
}

func TestSQLite_DeleteMessageInvalidHandle(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	err := s.DeleteMessage(context.Background(), "invalid-handle")
	assert.Error(t, err)
}

func TestSQLite_ReceiveMultipleMessages(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	for i := 0; i < 5; i++ {
		msg := &types.Message{
			MessageID: "msg-" + strconv.Itoa(i),
			Body:      "body",
		}
		require.NoError(t, s.SendMessage(context.Background(), msg, 0))
	}

	result, err := s.ReceiveMessages(context.Background(), 3, 30, 0)
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

// ---------------------------------------------------------------------------
// Visibility Timeout Tests (lazy — no goroutines)
// ---------------------------------------------------------------------------

func TestSQLite_VisibilityTimeoutExpiry(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	msg := &types.Message{MessageID: "msg-1", Body: "hello"}
	require.NoError(t, s.SendMessage(context.Background(), msg, 0))

	// Receive with 1 second visibility timeout
	result, err := s.ReceiveMessages(context.Background(), 1, 1, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	// Message should be in-flight
	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
	assert.Equal(t, 1, s.ApproximateNumberOfMessagesNotVisible())

	// Wait for visibility timeout to expire
	time.Sleep(2 * time.Second)

	// Message should be visible again (lazy check on next ReceiveMessages)
	assert.Equal(t, 1, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())

	// Should be able to receive it again
	result, err = s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "msg-1", result[0].MessageID)
	assert.Equal(t, 2, result[0].ApproximateReceiveCount)
}

func TestSQLite_ChangeMessageVisibility(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	msg := &types.Message{MessageID: "msg-1", Body: "hello"}
	require.NoError(t, s.SendMessage(context.Background(), msg, 0))

	result, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	// Change visibility to 0 (immediately visible)
	err = s.ChangeMessageVisibility(context.Background(), result[0].ReceiptHandle, 0)
	require.NoError(t, err)

	assert.Equal(t, 1, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())
}

// ---------------------------------------------------------------------------
// Purge and Count Tests
// ---------------------------------------------------------------------------

func TestSQLite_Purge(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	for i := 0; i < 5; i++ {
		msg := &types.Message{
			MessageID: "msg-" + strconv.Itoa(i),
			Body:      "body",
		}
		require.NoError(t, s.SendMessage(context.Background(), msg, 0))
	}

	assert.Equal(t, 5, s.ApproximateNumberOfMessages())

	err := s.Purge(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
}

// ---------------------------------------------------------------------------
// Long Polling Tests
// ---------------------------------------------------------------------------

func TestSQLite_LongPolling(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	// Send a message after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		msg := &types.Message{MessageID: "msg-1", Body: "hello"}
		_ = s.SendMessage(context.Background(), msg, 0)
	}()

	// Long poll with 2 second wait
	result, err := s.ReceiveMessages(context.Background(), 1, 30, 2)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "msg-1", result[0].MessageID)
}

func TestSQLite_LongPollingTimeout(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	start := time.Now()
	result, err := s.ReceiveMessages(context.Background(), 1, 30, 1)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Empty(t, result)
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond)
}

// ---------------------------------------------------------------------------
// Concurrent Access Tests
// ---------------------------------------------------------------------------

func TestSQLite_ConcurrentAccess(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	var wg sync.WaitGroup

	// Concurrent senders
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			msg := &types.Message{
				MessageID: "msg-" + strconv.Itoa(n),
				Body:      "body",
			}
			_ = s.SendMessage(context.Background(), msg, 0)
		}(i)
	}

	// Concurrent receivers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.ReceiveMessages(context.Background(), 1, 30, 0)
		}()
	}

	wg.Wait()
}

// ---------------------------------------------------------------------------
// Receipt Handle Tests
// ---------------------------------------------------------------------------

func TestSQLite_ReceiptHandleFormat(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	defer s.Close()

	msg := &types.Message{MessageID: "msg-1", Body: "hello"}
	require.NoError(t, s.SendMessage(context.Background(), msg, 0))

	result, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.NotEmpty(t, result[0].ReceiptHandle)
}

// ---------------------------------------------------------------------------
// Multiple Queues Same DB Tests
// ---------------------------------------------------------------------------

func TestSQLite_MultipleQueuesSameDB(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	s1, err := sqlite.NewSQLiteStore(db, "queue-1", 30, []byte("secret"), store.StoreConfig{})
	require.NoError(t, err)
	defer s1.Close()

	s2, err := sqlite.NewSQLiteStore(db, "queue-2", 30, []byte("secret"), store.StoreConfig{})
	require.NoError(t, err)
	defer s2.Close()

	msg1 := &types.Message{MessageID: "msg-q1", Body: "q1"}
	require.NoError(t, s1.SendMessage(context.Background(), msg1, 0))

	msg2 := &types.Message{MessageID: "msg-q2", Body: "q2"}
	require.NoError(t, s2.SendMessage(context.Background(), msg2, 0))

	assert.Equal(t, 1, s1.ApproximateNumberOfMessages())
	assert.Equal(t, 1, s2.ApproximateNumberOfMessages())

	// Receiving from queue-1 should not return queue-2's messages
	result, err := s1.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "msg-q1", result[0].MessageID)

	// Queue-2 should still have its message
	assert.Equal(t, 1, s2.ApproximateNumberOfMessages())
}
