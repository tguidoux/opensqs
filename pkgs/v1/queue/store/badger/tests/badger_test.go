package badger_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/badger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func newTestStore(t *testing.T) (*badger.BadgerStore, func()) {
	t.Helper()

	db, err := badger.Open(t.TempDir())
	require.NoError(t, err)

	s, err := badger.NewBadgerStore(db, "test-queue", 30, []byte("test-secret"), store.StoreConfig{})
	require.NoError(t, err)

	cleanup := func() {
		s.Close()
		db.Close()
	}

	return s, cleanup
}

func TestBadgerSendMessage(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

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

func TestBadgerSendMessageDelayed(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

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

func TestBadgerReceiveMessage(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
	}

	err := s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	msgs, err := s.ReceiveMessages(context.Background(), 10, 30, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	assert.Equal(t, "msg-1", msgs[0].MessageID)
	assert.Equal(t, "hello", msgs[0].Body)
	assert.NotEmpty(t, msgs[0].ReceiptHandle)
	assert.Equal(t, 1, msgs[0].ApproximateReceiveCount)

	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
	assert.Equal(t, 1, s.ApproximateNumberOfMessagesNotVisible())
}

func TestBadgerDeleteMessage(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
	}

	err := s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	msgs, err := s.ReceiveMessages(context.Background(), 10, 30, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	err = s.DeleteMessage(context.Background(), msgs[0].ReceiptHandle)
	require.NoError(t, err)

	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())
}

func TestBadgerDeleteMessageInvalidHandle(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	err := s.DeleteMessage(context.Background(), "invalid-handle")
	assert.Error(t, err)
}

func TestBadgerChangeMessageVisibility(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
	}

	err := s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	msgs, err := s.ReceiveMessages(context.Background(), 10, 30, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	assert.Equal(t, 1, s.ApproximateNumberOfMessagesNotVisible())

	// Make visible immediately
	err = s.ChangeMessageVisibility(context.Background(), msgs[0].ReceiptHandle, 0)
	require.NoError(t, err)

	assert.Equal(t, 1, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())
}

func TestBadgerChangeMessageVisibilityInvalidHandle(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	err := s.ChangeMessageVisibility(context.Background(), "invalid-handle", 10)
	assert.Error(t, err)
}

func TestBadgerPurge(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	for i := 0; i < 5; i++ {
		msg := &types.Message{
			MessageID: "msg-" + strconv.Itoa(i),
			Body:      "hello",
		}
		err := s.SendMessage(context.Background(), msg, 0)
		require.NoError(t, err)
	}

	assert.Equal(t, 5, s.ApproximateNumberOfMessages())

	err := s.Purge(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 0, s.ApproximateNumberOfMessages())
}

func TestBadgerReceiveEmpty(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	msgs, err := s.ReceiveMessages(context.Background(), 10, 30, 0)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestBadgerReceiveMultiple(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	for i := 0; i < 5; i++ {
		msg := &types.Message{
			MessageID: "msg-" + strconv.Itoa(i),
			Body:      "hello",
		}
		err := s.SendMessage(context.Background(), msg, 0)
		require.NoError(t, err)
	}

	msgs, err := s.ReceiveMessages(context.Background(), 3, 30, 0)
	require.NoError(t, err)
	assert.Len(t, msgs, 3)
}

func TestBadgerFIFODedup(t *testing.T) {
	db, err := badger.Open(t.TempDir())
	require.NoError(t, err)
	defer db.Close()

	s, err := badger.NewBadgerStore(db, "test.fifo", 30, []byte("test-secret"), store.StoreConfig{
		IsFifo: true,
	})
	require.NoError(t, err)
	defer s.Close()

	msg := &types.Message{
		MessageID:              "msg-1",
		Body:                   "hello",
		MessageDeduplicationID: "dedup-1",
		MessageGroupID:         "group-1",
	}

	err = s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	// Send duplicate
	err = s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	// Should only have 1 message
	assert.Equal(t, 1, s.ApproximateNumberOfMessages())
}

func TestBadgerConcurrentSendReceive(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	var wg sync.WaitGroup

	// Senders
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			msg := &types.Message{
				MessageID: "msg-" + strconv.Itoa(n),
				Body:      "hello",
			}
			_ = s.SendMessage(context.Background(), msg, 0)
		}(i)
	}

	wg.Wait()

	assert.Equal(t, 10, s.ApproximateNumberOfMessages())

	// Receivers
	var received int
	var mu sync.Mutex
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msgs, _ := s.ReceiveMessages(context.Background(), 2, 30, 0)
			mu.Lock()
			received += len(msgs)
			mu.Unlock()
		}()
	}

	wg.Wait()

	assert.Equal(t, 10, received)
}

func TestBadgerMessageAttributes(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
		MessageAttributes: map[string]types.MessageAttribute{
			"attr1": {
				DataType:    "String",
				StringValue: "value1",
			},
		},
	}

	err := s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	msgs, err := s.ReceiveMessages(context.Background(), 10, 30, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	assert.Equal(t, "value1", msgs[0].MessageAttributes["attr1"].StringValue)
}

func TestBadgerVisibilityTimeout(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	msg := &types.Message{
		MessageID: "msg-1",
		Body:      "hello",
	}

	err := s.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)

	// Receive with 1 second visibility timeout
	msgs, err := s.ReceiveMessages(context.Background(), 10, 1, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	assert.Equal(t, 1, s.ApproximateNumberOfMessagesNotVisible())

	// Wait for visibility to expire
	time.Sleep(2 * time.Second)

	// Message should be visible again
	assert.Equal(t, 1, s.ApproximateNumberOfMessages())
	assert.Equal(t, 0, s.ApproximateNumberOfMessagesNotVisible())

	// Receive again — should get the same message with receiveCount=2
	msgs, err = s.ReceiveMessages(context.Background(), 10, 30, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, 2, msgs[0].ApproximateReceiveCount)
}
