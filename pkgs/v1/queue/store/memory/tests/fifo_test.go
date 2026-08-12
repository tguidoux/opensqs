package memory_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func newFifoStore(t *testing.T) *memory.MemoryStore {
	t.Helper()
	return memory.NewMemoryStore("test-queue.fifo", 30, []byte("test-secret"), store.StoreConfig{
		IsFifo: true,
	})
}

func newFifoStoreWithDedup(t *testing.T) *memory.MemoryStore {
	t.Helper()
	return memory.NewMemoryStore("test-queue.fifo", 30, []byte("test-secret"), store.StoreConfig{
		IsFifo:                    true,
		ContentBasedDeduplication: true,
	})
}

// ---------------------------------------------------------------------------
// FIFO Message Group Ordering Tests
// ---------------------------------------------------------------------------

func TestFIFO_MessageGroupOrdering(t *testing.T) {
	s := newFifoStore(t)
	defer s.Close()

	// Send 3 messages in the same group
	for i := 0; i < 3; i++ {
		msg := &types.Message{
			MessageID:              "msg-" + strconv.Itoa(i),
			Body:                   "body-" + strconv.Itoa(i),
			MessageGroupID:         "groupA",
			MessageDeduplicationID: "dedup-" + strconv.Itoa(i),
		}
		err := s.SendMessage(context.Background(), msg, 0)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, s.ApproximateNumberOfMessages())

	// Receive first message — should be msg-0 (FIFO order)
	firstResult, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, firstResult, 1)
	assert.Equal(t, "msg-0", firstResult[0].MessageID)

	// Try to receive again — should return empty (groupA has in-flight message)
	result, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	assert.Empty(t, result)

	// Delete the first message to free the group
	// We need the receipt handle from the first receive — re-receive is empty,
	// so we use the handle from the first receive above
	err = s.DeleteMessage(context.Background(), firstResult[0].ReceiptHandle)
	require.NoError(t, err)

	// Now receive second message — should be msg-1
	result, err = s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "msg-1", result[0].MessageID)
}

func TestFIFO_MultipleGroupsParallel(t *testing.T) {
	s := newFifoStore(t)
	defer s.Close()

	// Send messages to two different groups
	msgA1 := &types.Message{MessageID: "a1", Body: "a1", MessageGroupID: "groupA", MessageDeduplicationID: "d-a1"}
	msgA2 := &types.Message{MessageID: "a2", Body: "a2", MessageGroupID: "groupA", MessageDeduplicationID: "d-a2"}
	msgB1 := &types.Message{MessageID: "b1", Body: "b1", MessageGroupID: "groupB", MessageDeduplicationID: "d-b1"}

	require.NoError(t, s.SendMessage(context.Background(), msgA1, 0))
	require.NoError(t, s.SendMessage(context.Background(), msgB1, 0))
	require.NoError(t, s.SendMessage(context.Background(), msgA2, 0))

	// Receive 2 messages — should get one from each group
	result, err := s.ReceiveMessages(context.Background(), 2, 30, 0)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// Verify we got one from each group
	groupIDs := map[string]bool{}
	for _, msg := range result {
		groupIDs[msg.MessageGroupID] = true
	}
	assert.Len(t, groupIDs, 2)
	assert.True(t, groupIDs["groupA"])
	assert.True(t, groupIDs["groupB"])
}

func TestFIFO_OneInFlightPerGroup(t *testing.T) {
	s := newFifoStore(t)
	defer s.Close()

	// Send 2 messages to same group
	msg1 := &types.Message{MessageID: "m1", Body: "m1", MessageGroupID: "groupA", MessageDeduplicationID: "d1"}
	msg2 := &types.Message{MessageID: "m2", Body: "m2", MessageGroupID: "groupA", MessageDeduplicationID: "d2"}

	require.NoError(t, s.SendMessage(context.Background(), msg1, 0))
	require.NoError(t, s.SendMessage(context.Background(), msg2, 0))

	// Receive — should get only 1 (first message from groupA)
	result, err := s.ReceiveMessages(context.Background(), 10, 30, 0)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "m1", result[0].MessageID)

	// Second receive should return empty (groupA still in-flight)
	result, err = s.ReceiveMessages(context.Background(), 10, 30, 0)
	require.NoError(t, err)
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// FIFO Deduplication Tests
// ---------------------------------------------------------------------------

func TestFIFO_DedupWithinWindow(t *testing.T) {
	s := newFifoStore(t)
	defer s.Close()

	msg1 := &types.Message{
		MessageID:              "msg-1",
		Body:                   "body-1",
		MessageGroupID:         "groupA",
		MessageDeduplicationID: "same-dedup-id",
	}
	err := s.SendMessage(context.Background(), msg1, 0)
	require.NoError(t, err)

	// Send another message with the same dedup ID — should be silently deduplicated
	msg2 := &types.Message{
		MessageID:              "msg-2",
		Body:                   "body-2",
		MessageGroupID:         "groupA",
		MessageDeduplicationID: "same-dedup-id",
	}
	err = s.SendMessage(context.Background(), msg2, 0)
	require.NoError(t, err)

	// Only 1 message should be in the queue
	assert.Equal(t, 1, s.ApproximateNumberOfMessages())

	// Receive should return the first message
	result, err := s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "msg-1", result[0].MessageID)
}

func TestFIFO_ContentBasedDeduplication(t *testing.T) {
	s := newFifoStoreWithDedup(t)
	defer s.Close()

	// With content-based dedup, same body = same dedup ID (SHA-256 of body)
	msg1 := &types.Message{
		MessageID:      "msg-1",
		Body:           "same-body",
		MessageGroupID: "groupA",
	}
	err := s.SendMessage(context.Background(), msg1, 0)
	require.NoError(t, err)

	// Same body, different message ID — should be deduplicated
	msg2 := &types.Message{
		MessageID:      "msg-2",
		Body:           "same-body",
		MessageGroupID: "groupA",
	}
	err = s.SendMessage(context.Background(), msg2, 0)
	require.NoError(t, err)

	assert.Equal(t, 1, s.ApproximateNumberOfMessages())

	// Different body — should NOT be deduplicated
	msg3 := &types.Message{
		MessageID:      "msg-3",
		Body:           "different-body",
		MessageGroupID: "groupA",
	}
	err = s.SendMessage(context.Background(), msg3, 0)
	require.NoError(t, err)

	assert.Equal(t, 2, s.ApproximateNumberOfMessages())
}

func TestFIFO_DedupExpiry(t *testing.T) {
	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	store.SetNowFunc(func() time.Time { return baseTime })
	defer store.SetNowFunc(func() time.Time { return time.Now().UTC() })

	s := newFifoStore(t)
	defer s.Close()

	msg1 := &types.Message{
		MessageID:              "msg-1",
		Body:                   "body-1",
		MessageGroupID:         "groupA",
		MessageDeduplicationID: "dedup-id",
	}
	require.NoError(t, s.SendMessage(context.Background(), msg1, 0))

	// Advance time past 5-minute dedup window
	baseTime = baseTime.Add(6 * time.Minute)

	// Same dedup ID should now be accepted
	msg2 := &types.Message{
		MessageID:              "msg-2",
		Body:                   "body-2",
		MessageGroupID:         "groupA",
		MessageDeduplicationID: "dedup-id",
	}
	require.NoError(t, s.SendMessage(context.Background(), msg2, 0))

	assert.Equal(t, 2, s.ApproximateNumberOfMessages())
}

// ---------------------------------------------------------------------------
// FIFO Sequence Number Tests
// ---------------------------------------------------------------------------

func TestFIFO_SequenceNumbers(t *testing.T) {
	s := newFifoStore(t)
	defer s.Close()

	msg1 := &types.Message{MessageID: "m1", Body: "b1", MessageGroupID: "g1", MessageDeduplicationID: "d1"}
	msg2 := &types.Message{MessageID: "m2", Body: "b2", MessageGroupID: "g1", MessageDeduplicationID: "d2"}

	require.NoError(t, s.SendMessage(context.Background(), msg1, 0))
	require.NoError(t, s.SendMessage(context.Background(), msg2, 0))

	// Both messages should have non-empty sequence numbers
	assert.NotEmpty(t, msg1.SequenceNumber)
	assert.NotEmpty(t, msg2.SequenceNumber)

	// Sequence numbers should be monotonically increasing
	assert.NotEqual(t, msg1.SequenceNumber, msg2.SequenceNumber)
}

// ---------------------------------------------------------------------------
// FIFO Visibility Timeout Re-queue Tests
// ---------------------------------------------------------------------------

func TestFIFO_VisibilityTimeoutRequeue(t *testing.T) {
	s := memory.NewMemoryStore("test-queue.fifo", 1, []byte("test-secret"), store.StoreConfig{IsFifo: true})
	defer s.Close()

	msg1 := &types.Message{MessageID: "m1", Body: "b1", MessageGroupID: "g1", MessageDeduplicationID: "d1"}
	msg2 := &types.Message{MessageID: "m2", Body: "b2", MessageGroupID: "g1", MessageDeduplicationID: "d2"}

	require.NoError(t, s.SendMessage(context.Background(), msg1, 0))
	require.NoError(t, s.SendMessage(context.Background(), msg2, 0))

	// Receive first message
	result, err := s.ReceiveMessages(context.Background(), 1, 1, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "m1", result[0].MessageID)

	// Wait for visibility timeout to expire (real time, since time.AfterFunc uses real time)
	time.Sleep(1500 * time.Millisecond)

	// Should be able to receive m1 again (visibility expired, back in group)
	result, err = s.ReceiveMessages(context.Background(), 1, 30, 0)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "m1", result[0].MessageID)
}
