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

// TestDLQ_RedriveAfterMaxReceiveCount verifies that a message is redrived
// to the DLQ after exceeding maxReceiveCount.
func TestDLQ_RedriveAfterMaxReceiveCount(t *testing.T) {
	var redrivedMessages []*types.Message
	var mu sync.Mutex

	redriveFunc := func(msg *types.Message) {
		mu.Lock()
		defer mu.Unlock()
		redrivedMessages = append(redrivedMessages, msg)
	}

	// Use real time — time.AfterFunc fires based on real time, not store.Now
	s := memory.NewMemoryStore("main-queue", 1, []byte("test-secret"), store.StoreConfig{
		MaxReceiveCount: 3,
		RedriveFunc:     redriveFunc,
	})
	defer s.Close()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
	}
	require.NoError(t, s.SendMessage(context.Background(), msg, 0))

	// Receive and let visibility timeout expire 3 times
	for i := 0; i < 3; i++ {
		result, err := s.ReceiveMessages(context.Background(), 1, 1, 0)
		require.NoError(t, err)
		require.Len(t, result, 1, "should receive message on attempt %d", i+1)

		// Wait for the 1-second visibility timeout to expire and the timer callback to fire
		time.Sleep(1500 * time.Millisecond)
	}

	// Give the timer callback a moment to execute
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, redrivedMessages, 1)
	assert.Equal(t, "msg-1", redrivedMessages[0].MessageID)

	// Message should no longer be in the main queue
	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())
}

// TestDLQ_NoRedriveBelowMaxReceiveCount verifies that messages are NOT redrived
// when receive count is below maxReceiveCount.
func TestDLQ_NoRedriveBelowMaxReceiveCount(t *testing.T) {
	var redrivedCount int
	var mu sync.Mutex

	redriveFunc := func(msg *types.Message) {
		mu.Lock()
		defer mu.Unlock()
		redrivedCount++
	}

	s := memory.NewMemoryStore("main-queue", 1, []byte("test-secret"), store.StoreConfig{
		MaxReceiveCount: 5,
		RedriveFunc:     redriveFunc,
	})
	defer s.Close()

	msg := &types.Message{MessageID: "msg-1", Body: "hello"}
	require.NoError(t, s.SendMessage(context.Background(), msg, 0))

	// Receive and let visibility timeout expire twice (below max of 5)
	for i := 0; i < 2; i++ {
		result, err := s.ReceiveMessages(context.Background(), 1, 1, 0)
		require.NoError(t, err)
		require.Len(t, result, 1)

		// Wait for visibility timeout to expire
		time.Sleep(1500 * time.Millisecond)
	}

	// After 2 receives, message should be visible again (not redrived)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 0, redrivedCount)
	assert.Equal(t, 1, s.ApproximateNumberOfMessages())
}

// TestDLQ_RedriveOnVisibilityTimeout verifies that the visibility timer
// triggers redrive when maxReceiveCount is exceeded.
func TestDLQ_RedriveOnVisibilityTimeout(t *testing.T) {
	var redrivedMessages []*types.Message
	var mu sync.Mutex

	redriveFunc := func(msg *types.Message) {
		mu.Lock()
		defer mu.Unlock()
		redrivedMessages = append(redrivedMessages, msg)
	}

	s := memory.NewMemoryStore("main-queue", 1, []byte("test-secret"), store.StoreConfig{
		MaxReceiveCount: 2,
		RedriveFunc:     redriveFunc,
	})
	defer s.Close()

	msg := &types.Message{MessageID: "msg-1", Body: "hello"}
	require.NoError(t, s.SendMessage(context.Background(), msg, 0))

	// First receive
	result, err := s.ReceiveMessages(context.Background(), 1, 1, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	// Wait for visibility timeout (receiveCount=1, below max of 2)
	time.Sleep(1500 * time.Millisecond)

	// Second receive
	result, err = s.ReceiveMessages(context.Background(), 1, 1, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	// Let visibility timeout expire again (receiveCount=2, meets max of 2)
	// The timer callback should redrive the message
	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, redrivedMessages, 1)
	assert.Equal(t, "msg-1", redrivedMessages[0].MessageID)
}

// TestDLQ_NoRedriveFunc verifies that messages stay in the queue when no
// redrive function is configured (even if maxReceiveCount is exceeded).
func TestDLQ_NoRedriveFunc(t *testing.T) {
	s := memory.NewMemoryStore("main-queue", 1, []byte("test-secret"), store.StoreConfig{
		MaxReceiveCount: 1,
		// No RedriveFunc
	})
	defer s.Close()

	msg := &types.Message{MessageID: "msg-1", Body: "hello"}
	require.NoError(t, s.SendMessage(context.Background(), msg, 0))

	// Receive
	result, err := s.ReceiveMessages(context.Background(), 1, 1, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)

	// Let visibility timeout expire
	time.Sleep(1500 * time.Millisecond)

	// Message should still be in the queue (no redrive function to call)
	assert.Equal(t, 1, s.ApproximateNumberOfMessages())
}
