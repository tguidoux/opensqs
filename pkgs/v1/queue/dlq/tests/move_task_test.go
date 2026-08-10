package dlq_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/dlq"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// mockQueueRef is a test implementation of dlq.QueueRef.
type mockQueueRef struct {
	arn           string
	redrivePolicy string
	store         *memory.MemoryStore
}

func (m *mockQueueRef) Store() store.Store {
	return m.store
}

func (m *mockQueueRef) GetQueueArn() string {
	return m.arn
}

func (m *mockQueueRef) GetRedrivePolicy() string {
	return m.redrivePolicy
}

// newMockQueue creates a mock QueueRef with a memory store.
func newMockQueue(name, arn, redrivePolicy string) *mockQueueRef {
	s := memory.NewMemoryStore(name, 30, []byte("test-secret"), storeConfig())
	return &mockQueueRef{
		arn:           arn,
		redrivePolicy: redrivePolicy,
		store:         s,
	}
}

// storeConfig returns a default store config for memory store.
func storeConfig() store.StoreConfig {
	return store.StoreConfig{}
}

// newTestManager creates a MoveTaskManager with the given queues.
func newTestManager(queues ...*mockQueueRef) (*dlq.MoveTaskManager, map[string]*mockQueueRef) {
	queueMap := make(map[string]*mockQueueRef)
	for _, q := range queues {
		queueMap[q.arn] = q
	}

	lookupFn := func(arn string) (dlq.QueueRef, error) {
		q, ok := queueMap[arn]
		if !ok {
			return nil, fmt.Errorf("queue not found: %s", arn)
		}
		return q, nil
	}
	listFn := func(prefix string) []dlq.QueueRef {
		result := make([]dlq.QueueRef, 0, len(queues))
		for _, q := range queues {
			result = append(result, q)
		}
		return result
	}

	return dlq.NewMoveTaskManager(lookupFn, listFn), queueMap
}

// sendMessageToQueue sends a message to a mock queue's store.
func sendMessageToQueue(t *testing.T, q *mockQueueRef, body string) {
	t.Helper()
	msg := &types.Message{
		MessageID: fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		Body:      body,
		IsVisible: true,
	}
	err := q.store.SendMessage(context.Background(), msg, 0)
	require.NoError(t, err)
}

// receiveFromQueue receives messages from a mock queue's store.
func receiveFromQueue(t *testing.T, q *mockQueueRef, max int) []*types.Message {
	t.Helper()
	msgs, err := q.store.ReceiveMessages(context.Background(), max, 1, 0)
	require.NoError(t, err)
	return msgs
}

// ---------------------------------------------------------------------------
// StartTask Tests
// ---------------------------------------------------------------------------

func TestStartTask_MissingSourceArn(t *testing.T) {
	mtm, _ := newTestManager()
	_, err := mtm.StartTask("", "", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source ARN")
}

func TestStartTask_SourceQueueNotFound(t *testing.T) {
	mtm, _ := newTestManager()
	_, err := mtm.StartTask("arn:aws:sqs:us-east-1:123456789012:nonexistent", "", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source queue not found")
}

func TestStartTask_DestinationQueueNotFound(t *testing.T) {
	src := newMockQueue("src", "arn:aws:sqs:us-east-1:123456789012:src", "")
	mtm, _ := newTestManager(src)

	_, err := mtm.StartTask(
		"arn:aws:sqs:us-east-1:123456789012:src",
		"arn:aws:sqs:us-east-1:123456789012:nonexistent",
		0,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "destination queue not found")
}

func TestStartTask_WithExplicitDestination(t *testing.T) {
	src := newMockQueue("src", "arn:aws:sqs:us-east-1:123456789012:src", "")
	dst := newMockQueue("dst", "arn:aws:sqs:us-east-1:123456789012:dst", "")
	mtm, _ := newTestManager(src, dst)

	// Send 3 messages to source
	for i := 0; i < 3; i++ {
		sendMessageToQueue(t, src, fmt.Sprintf("msg-%d", i))
	}

	task, err := mtm.StartTask(
		"arn:aws:sqs:us-east-1:123456789012:src",
		"arn:aws:sqs:us-east-1:123456789012:dst",
		0,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, task.TaskHandle())
	assert.Equal(t, dlq.MoveTaskStatusRunning, task.Status())
	assert.Equal(t, "arn:aws:sqs:us-east-1:123456789012:src", task.SourceArn())
	assert.Equal(t, "arn:aws:sqs:us-east-1:123456789012:dst", task.DestinationArn())

	// Wait for task to complete
	time.Sleep(200 * time.Millisecond)

	// Verify messages were moved
	dstMsgs := receiveFromQueue(t, dst, 10)
	assert.Len(t, dstMsgs, 3)

	// Verify source is empty
	srcMsgs := receiveFromQueue(t, src, 10)
	assert.Empty(t, srcMsgs)

	// Verify task status
	task, ok := mtm.GetTask(task.TaskHandle())
	require.True(t, ok)
	assert.Equal(t, dlq.MoveTaskStatusCompleted, task.Status())
	assert.Equal(t, 3, task.MovedMessages())
}

func TestStartTask_AutoDiscoverDestination(t *testing.T) {
	dlqArn := "arn:aws:sqs:us-east-1:123456789012:my-dlq"
	mainArn := "arn:aws:sqs:us-east-1:123456789012:my-main"

	// Main queue has a redrive policy pointing to the DLQ
	rp := `{"deadLetterTargetArn":"` + dlqArn + `","maxReceiveCount":"3"}`
	mainQ := newMockQueue("my-main", mainArn, rp)
	dlqQ := newMockQueue("my-dlq", dlqArn, "")

	mtm, _ := newTestManager(mainQ, dlqQ)

	// Send messages to DLQ
	sendMessageToQueue(t, dlqQ, "dlq-msg-1")
	sendMessageToQueue(t, dlqQ, "dlq-msg-2")

	// Start task with empty destination — should auto-discover main queue
	task, err := mtm.StartTask(dlqArn, "", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, task.TaskHandle())
	assert.Equal(t, mainArn, task.DestinationArn())

	// Wait for task to complete
	time.Sleep(200 * time.Millisecond)

	// Verify messages were moved to main queue
	mainMsgs := receiveFromQueue(t, mainQ, 10)
	assert.Len(t, mainMsgs, 2)

	// Verify DLQ is empty
	dlqMsgs := receiveFromQueue(t, dlqQ, 10)
	assert.Empty(t, dlqMsgs)
}

func TestStartTask_AutoDiscoverNoSourceQueue(t *testing.T) {
	dlqArn := "arn:aws:sqs:us-east-1:123456789012:orphan-dlq"
	dlqQ := newMockQueue("orphan-dlq", dlqArn, "")

	// No queue has a redrive policy pointing to this DLQ
	mtm, _ := newTestManager(dlqQ)

	_, err := mtm.StartTask(dlqArn, "", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no source queue found")
}

func TestStartTask_RateLimiting(t *testing.T) {
	src := newMockQueue("src", "arn:aws:sqs:us-east-1:123456789012:src", "")
	dst := newMockQueue("dst", "arn:aws:sqs:us-east-1:123456789012:dst", "")
	mtm, _ := newTestManager(src, dst)

	// Send 3 messages to source
	for i := 0; i < 3; i++ {
		sendMessageToQueue(t, src, fmt.Sprintf("msg-%d", i))
	}

	// Start task with rate limit of 2 messages per second
	task, err := mtm.StartTask(
		"arn:aws:sqs:us-east-1:123456789012:src",
		"arn:aws:sqs:us-east-1:123456789012:dst",
		2,
	)
	require.NoError(t, err)

	// After 100ms, at most ~1 message should have been moved (2/sec = 500ms per msg)
	time.Sleep(100 * time.Millisecond)

	task, _ = mtm.GetTask(task.TaskHandle())
	assert.LessOrEqual(t, task.MovedMessages(), 1)

	// Poll for completion — 3 messages at 2/sec = ~1.5s, but allow up to 10s
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		task, _ = mtm.GetTask(task.TaskHandle())
		if task.Status() == dlq.MoveTaskStatusCompleted {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	task, _ = mtm.GetTask(task.TaskHandle())
	assert.Equal(t, dlq.MoveTaskStatusCompleted, task.Status())
	assert.Equal(t, 3, task.MovedMessages())
}

// ---------------------------------------------------------------------------
// CancelTask Tests
// ---------------------------------------------------------------------------

func TestCancelTask_NotFound(t *testing.T) {
	mtm, _ := newTestManager()
	_, err := mtm.CancelTask("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")
}

func TestCancelTask_Success(t *testing.T) {
	src := newMockQueue("src", "arn:aws:sqs:us-east-1:123456789012:src", "")
	dst := newMockQueue("dst", "arn:aws:sqs:us-east-1:123456789012:dst", "")
	mtm, _ := newTestManager(src, dst)

	// Send 5 messages to source
	for i := 0; i < 5; i++ {
		sendMessageToQueue(t, src, fmt.Sprintf("msg-%d", i))
	}

	// Start task with rate limiting so we can cancel it
	task, err := mtm.StartTask(
		"arn:aws:sqs:us-east-1:123456789012:src",
		"arn:aws:sqs:us-east-1:123456789012:dst",
		1, // 1 msg/sec
	)
	require.NoError(t, err)

	// Wait a bit for some messages to move
	time.Sleep(100 * time.Millisecond)

	// Cancel the task
	moved, err := mtm.CancelTask(task.TaskHandle())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, moved, 0)

	// Wait for the background goroutine to process the cancellation
	// and update the status from CANCELLING to CANCELLED
	require.Eventually(t, func() bool {
		task, ok := mtm.GetTask(task.TaskHandle())
		return ok && task.Status() == dlq.MoveTaskStatusCancelled
	}, 2*time.Second, 10*time.Millisecond)

	// Verify task status
	task, ok := mtm.GetTask(task.TaskHandle())
	require.True(t, ok)
	assert.Equal(t, dlq.MoveTaskStatusCancelled, task.Status())
}

func TestCancelTask_AlreadyCancelled(t *testing.T) {
	src := newMockQueue("src", "arn:aws:sqs:us-east-1:123456789012:src", "")
	dst := newMockQueue("dst", "arn:aws:sqs:us-east-1:123456789012:dst", "")
	mtm, _ := newTestManager(src, dst)

	task, err := mtm.StartTask(
		"arn:aws:sqs:us-east-1:123456789012:src",
		"arn:aws:sqs:us-east-1:123456789012:dst",
		1,
	)
	require.NoError(t, err)

	// Cancel once
	_, err = mtm.CancelTask(task.TaskHandle())
	require.NoError(t, err)

	// Cancel again — should not error, just return current count
	moved, err := mtm.CancelTask(task.TaskHandle())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, moved, 0)
}

// ---------------------------------------------------------------------------
// ListTasks Tests
// ---------------------------------------------------------------------------

func TestListTasks_Empty(t *testing.T) {
	mtm, _ := newTestManager()
	tasks := mtm.ListTasks("arn:aws:sqs:us-east-1:123456789012:any")
	assert.Empty(t, tasks)
}

func TestListTasks_FilterBySourceArn(t *testing.T) {
	src1 := newMockQueue("src1", "arn:aws:sqs:us-east-1:123456789012:src1", "")
	src2 := newMockQueue("src2", "arn:aws:sqs:us-east-1:123456789012:src2", "")
	dst := newMockQueue("dst", "arn:aws:sqs:us-east-1:123456789012:dst", "")
	mtm, _ := newTestManager(src1, src2, dst)

	// Send plenty of messages to both sources so tasks stay running
	for i := 0; i < 20; i++ {
		sendMessageToQueue(t, src1, fmt.Sprintf("src1-msg-%d", i))
	}
	for i := 0; i < 10; i++ {
		sendMessageToQueue(t, src2, fmt.Sprintf("src2-msg-%d", i))
	}

	// Start tasks for src1 with rate limiting to keep them running
	task1, err := mtm.StartTask(
		"arn:aws:sqs:us-east-1:123456789012:src1",
		"arn:aws:sqs:us-east-1:123456789012:dst",
		1,
	)
	require.NoError(t, err)

	// List immediately — task1 should still be running
	src1Tasks := mtm.ListTasks("arn:aws:sqs:us-east-1:123456789012:src1")
	assert.Len(t, src1Tasks, 1)

	task2, err := mtm.StartTask(
		"arn:aws:sqs:us-east-1:123456789012:src1",
		"arn:aws:sqs:us-east-1:123456789012:dst",
		1,
	)
	require.NoError(t, err)

	// Start task for src2
	task3, err := mtm.StartTask(
		"arn:aws:sqs:us-east-1:123456789012:src2",
		"arn:aws:sqs:us-east-1:123456789012:dst",
		1,
	)
	require.NoError(t, err)

	// List tasks for src1 — should have 2
	src1Tasks = mtm.ListTasks("arn:aws:sqs:us-east-1:123456789012:src1")
	assert.Len(t, src1Tasks, 2)

	// List tasks for src2
	src2Tasks := mtm.ListTasks("arn:aws:sqs:us-east-1:123456789012:src2")
	assert.Len(t, src2Tasks, 1)
	assert.Equal(t, task3.TaskHandle(), src2Tasks[0].TaskHandle())

	// List all tasks
	allTasks := mtm.ListTasks("")
	assert.Len(t, allTasks, 3)

	// Verify task handles
	handles := map[string]bool{task1.TaskHandle(): true, task2.TaskHandle(): true, task3.TaskHandle(): true}
	for _, task := range allTasks {
		assert.True(t, handles[task.TaskHandle()], "unexpected task handle: %s", task.TaskHandle())
	}
}

// ---------------------------------------------------------------------------
// GetTask Tests
// ---------------------------------------------------------------------------

func TestGetTask_NotFound(t *testing.T) {
	mtm, _ := newTestManager()
	_, ok := mtm.GetTask("nonexistent")
	assert.False(t, ok)
}

func TestGetTask_Found(t *testing.T) {
	src := newMockQueue("src", "arn:aws:sqs:us-east-1:123456789012:src", "")
	dst := newMockQueue("dst", "arn:aws:sqs:us-east-1:123456789012:dst", "")
	mtm, _ := newTestManager(src, dst)

	task, err := mtm.StartTask(
		"arn:aws:sqs:us-east-1:123456789012:src",
		"arn:aws:sqs:us-east-1:123456789012:dst",
		0,
	)
	require.NoError(t, err)

	found, ok := mtm.GetTask(task.TaskHandle())
	require.True(t, ok)
	assert.Equal(t, task.TaskHandle(), found.TaskHandle())
	assert.Equal(t, task.SourceArn(), found.SourceArn())
	assert.Equal(t, task.DestinationArn(), found.DestinationArn())
}

// ---------------------------------------------------------------------------
// Concurrent Access Tests
// ---------------------------------------------------------------------------

func TestMoveTaskManager_ConcurrentStartTasks(t *testing.T) {
	src := newMockQueue("src", "arn:aws:sqs:us-east-1:123456789012:src", "")
	dst := newMockQueue("dst", "arn:aws:sqs:us-east-1:123456789012:dst", "")
	mtm, _ := newTestManager(src, dst)

	// Send messages so tasks don't complete immediately
	for i := 0; i < 10; i++ {
		sendMessageToQueue(t, src, fmt.Sprintf("msg-%d", i))
	}

	var wg sync.WaitGroup
	handles := make([]string, 5)
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			task, err := mtm.StartTask(
				"arn:aws:sqs:us-east-1:123456789012:src",
				"arn:aws:sqs:us-east-1:123456789012:dst",
				1, // rate limit to keep tasks running
			)
			require.NoError(t, err)
			mu.Lock()
			handles[idx] = task.TaskHandle()
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// All handles should be unique
	unique := make(map[string]bool)
	for _, h := range handles {
		assert.False(t, unique[h], "duplicate task handle: %s", h)
		unique[h] = true
	}

	// Should have 5 tasks
	allTasks := mtm.ListTasks("")
	assert.Len(t, allTasks, 5)
}
