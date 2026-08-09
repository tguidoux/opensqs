package memory_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func newTestStore(t *testing.T) *memory.MemoryStore {
	t.Helper()
	return memory.NewMemoryStore("test-queue", 30, []byte("test-secret"), store.StoreConfig{})
}

func TestSendMessage(t *testing.T) {
	s := newTestStore(t)
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

func TestSendMessageDelayed(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
	}

	err := s.SendMessage(context.Background(), msg, 60)
	require.NoError(t, err)

	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())
	assert.Equal(t, 1, s.ApproximateNumberOfMessagesDelayed())
}

func TestReceiveMessage(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
	}

	err := s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	result, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "msg-1", result[0].MessageID)
	assert.NotEmpty(t, result[0].ReceiptHandle)
	assert.Equal(t, 1, result[0].ApproximateReceiveCount)

	// Message should be in-flight now
	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
	assert.Equal(t, 1, s.ApproximateNumberOfMessagesNotVisible())
}

func TestReceiveMessageEmpty(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	result, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestDeleteMessage(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
	}

	err := s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	result, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	err = s.DeleteMessage(context.Background(), result[0].ReceiptHandle)
	require.NoError(t, err)

	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())
}

func TestDeleteMessageInvalidHandle(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	err := s.DeleteMessage(context.Background(), "invalid-handle")
	assert.Error(t, err)
}

func TestReceiveMultipleMessages(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	for i := 0; i < 5; i++ {
		msg := &types.Message{
			MessageID: "msg-" + string(rune('0'+i)),
			Body:      "body",
		}
		err := s.SendMessage(context.Background(), msg, 0)
		require.NoError(t, err)
	}

	result, err := s.ReceiveMessages(context.Background(), 3, 30, 0)
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestVisibilityTimeoutExpiry(t *testing.T) {
	// Override time for this test
	originalNow := store.Now
	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return baseTime }
	defer func() { store.Now = originalNow }()

	s := memory.NewMemoryStore("test-queue", 1, []byte("test-secret"), store.StoreConfig{})
	defer s.Close()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
	}

	err := s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	// Receive the message with 1 second visibility timeout
	result, err := s.ReceiveMessages(context.Background(), 1, 1, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	// Message should be in-flight
	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
	assert.Equal(t, 1, s.ApproximateNumberOfMessagesNotVisible())

	// Advance time past visibility timeout
	baseTime = baseTime.Add(2 * time.Second)
	time.Sleep(50 * time.Millisecond) // Allow timer goroutine to run

	// Message should be visible again
	assert.Equal(t, 1, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())
}

func TestChangeMessageVisibility(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
	}

	err := s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	result, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	// Change visibility to 0 (immediately visible)
	err = s.ChangeMessageVisibility(context.Background(), result[0].ReceiptHandle, 0)
	require.NoError(t, err)

	assert.Equal(t, 1, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())
}

func TestPurge(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	for i := 0; i < 5; i++ {
		msg := &types.Message{
			MessageID: "msg-" + string(rune('0'+i)),
			Body:      "body",
		}
		err := s.SendMessage(context.Background(), msg, 0)
		require.NoError(t, err)
	}

	assert.Equal(t, 5, s.ApproximateNumberOfMessages())

	err := s.Purge(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
}

func TestConcurrentAccess(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	var wg sync.WaitGroup

	// Concurrent senders
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			msg := &types.Message{
				MessageID: "msg-" + string(rune('0'+n)),
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

func TestLongPolling(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Send a message after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		msg := &types.Message{
			MessageID: "msg-1",
			Body:      "hello",
		}
		_ = s.SendMessage(context.Background(), msg, 0)
	}()

	// Long poll with 2 second wait
	result, err := s.ReceiveMessages(context.Background(), 1, 30, 2)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "msg-1", result[0].MessageID)
}

func TestLongPollingTimeout(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	start := time.Now()
	result, err := s.ReceiveMessages(context.Background(), 1, 30, 1)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Empty(t, result)
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond)
}

func TestReceiptHandleFormat(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
	}

	err := s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	result, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	// Receipt handle should be a non-empty base64 string
	assert.NotEmpty(t, result[0].ReceiptHandle)
}
