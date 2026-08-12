package dlq

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tguidoux/opensqs/pkgs/v1/id"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
)

// QueueRef is a minimal interface representing a queue that can be used for move tasks.
// *queue.Queue satisfies this interface.
type QueueRef interface {
	Store() store.Store
	GetQueueArn() string
	GetRedrivePolicy() string
}

// QueueLookupFunc is a function type for looking up a queue by ARN.
// It returns any so that *queue.QueueManager can satisfy it without circular imports.
type QueueLookupFunc func(arn string) (QueueRef, error)

// QueueListFunc is a function type for listing queues by prefix.
// It returns any so that *queue.QueueManager can satisfy it without circular imports.
type QueueListFunc func(prefix string) []QueueRef

// MoveTaskStatus represents the current state of a message move task.
type MoveTaskStatus string

const (
	// MoveTaskStatusRunning indicates the task is actively moving messages.
	MoveTaskStatusRunning MoveTaskStatus = "RUNNING"
	// MoveTaskStatusCompleted indicates the task finished moving all messages.
	MoveTaskStatusCompleted MoveTaskStatus = "COMPLETED"
	// MoveTaskStatusCancelling indicates the task was requested to cancel.
	MoveTaskStatusCancelling MoveTaskStatus = "CANCELLING"
	// MoveTaskStatusCancelled indicates the task was cancelled.
	MoveTaskStatusCancelled MoveTaskStatus = "CANCELLED"
	// MoveTaskStatusFailed indicates the task encountered an error.
	MoveTaskStatusFailed MoveTaskStatus = "FAILED"
)

// MoveTask represents a single message move task that transfers messages
// from a source queue (typically a DLQ) to a destination queue.
// All fields are unexported; use the getter methods for thread-safe access.
type MoveTask struct {
	taskHandle                   string
	sourceArn                    string
	destinationArn               string
	status                       atomic.Value // stores MoveTaskStatus
	maxNumberOfMessagesPerSecond int
	movedMessages                atomic.Int64
	startedAt                    time.Time
	completedAt                  atomic.Value // stores time.Time
	cancelled                    chan struct{}
	cancelOnce                   sync.Once
}

// TaskHandle returns the unique handle for this task.
func (t *MoveTask) TaskHandle() string { return t.taskHandle }

// SourceArn returns the source queue ARN.
func (t *MoveTask) SourceArn() string { return t.sourceArn }

// DestinationArn returns the destination queue ARN.
func (t *MoveTask) DestinationArn() string { return t.destinationArn }

// Status returns the current task status.
// Thread-safe via atomic.Value.
func (t *MoveTask) Status() MoveTaskStatus {
	if v := t.status.Load(); v != nil {
		return v.(MoveTaskStatus)
	}
	return MoveTaskStatusFailed
}

// MaxNumberOfMessagesPerSecond returns the rate limit for this task.
func (t *MoveTask) MaxNumberOfMessagesPerSecond() int { return t.maxNumberOfMessagesPerSecond }

// MovedMessages returns the number of messages moved so far.
// This method is thread-safe.
func (t *MoveTask) MovedMessages() int {
	return int(t.movedMessages.Load())
}

// StartedAt returns the time the task was started.
func (t *MoveTask) StartedAt() time.Time { return t.startedAt }

// CompletedAt returns the time the task reached a terminal state (completed, cancelled, or failed).
// Returns the zero time if the task is still running.
func (t *MoveTask) CompletedAt() time.Time {
	if v := t.completedAt.Load(); v != nil {
		return v.(time.Time)
	}
	return time.Time{}
}

// taskTTL is how long a completed/cancelled/failed task is kept before cleanup.
const taskTTL = 24 * time.Hour

// taskCleanupInterval is how often the background cleanup runs.
const taskCleanupInterval = 1 * time.Hour

// MoveTaskManager manages the lifecycle of message move tasks.
// It tracks active and completed tasks, starts background goroutines
// to move messages, and supports cancellation.
// Completed tasks are automatically cleaned up after taskTTL.
type MoveTaskManager struct {
	mu          sync.RWMutex
	tasks       map[string]*MoveTask
	lookupFn    QueueLookupFunc
	listFn      QueueListFunc
	log         logger.LoggerInterface
	stopCleanup chan struct{}
	closeOnce   sync.Once
}

// NewMoveTaskManager creates a new MoveTaskManager wired to the given lookup and list functions.
// The lookup function resolves a queue by ARN, the list function lists queues by prefix.
// Both are typically wired from *queue.QueueManager.
func NewMoveTaskManager(lookupFn QueueLookupFunc, listFn QueueListFunc, log logger.LoggerInterface) *MoveTaskManager {
	mtm := &MoveTaskManager{
		tasks:       make(map[string]*MoveTask),
		lookupFn:    lookupFn,
		listFn:      listFn,
		log:         log,
		stopCleanup: make(chan struct{}),
	}
	go mtm.cleanupLoop()
	return mtm
}

// cleanupLoop periodically removes completed tasks older than taskTTL.
func (mtm *MoveTaskManager) cleanupLoop() {
	ticker := time.NewTicker(taskCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			mtm.removeStaleTasks()
		case <-mtm.stopCleanup:
			return
		}
	}
}

// removeStaleTasks removes completed/cancelled/failed tasks older than taskTTL.
func (mtm *MoveTaskManager) removeStaleTasks() {
	mtm.mu.Lock()
	defer mtm.mu.Unlock()
	now := time.Now()
	for handle, t := range mtm.tasks {
		status := t.Status()
		if status == MoveTaskStatusRunning || status == MoveTaskStatusCancelling {
			continue
		}
		// Use completedAt (when the task reached a terminal state) for TTL calculation.
		// Falls back to startedAt if completedAt was never set (defensive).
		reference := t.CompletedAt()
		if reference.IsZero() {
			reference = t.startedAt
		}
		if now.Sub(reference) > taskTTL {
			delete(mtm.tasks, handle)
		}
	}
}

// Close stops the cleanup goroutine. Call this when the manager is no longer needed.
// Safe to call multiple times.
func (mtm *MoveTaskManager) Close() {
	mtm.closeOnce.Do(func() {
		close(mtm.stopCleanup)
	})
}

// StartTask creates and starts a new message move task.
// If destinationArn is empty, messages are moved back to the source queue
// that originally sent them (the DLQ's source queue).
// If maxRate is > 0, message movement is rate-limited to that many messages per second.
func (mtm *MoveTaskManager) StartTask(sourceArn, destinationArn string, maxRate int) (*MoveTask, error) {
	if sourceArn == "" {
		return nil, fmt.Errorf("source ARN is required")
	}

	// Prevent moving messages to the same queue (no-op / confusing)
	if destinationArn != "" && sourceArn == destinationArn {
		return nil, fmt.Errorf("source and destination ARNs must be different")
	}

	// Verify source queue exists
	sourceQueue, err := mtm.lookupFn(sourceArn)
	if err != nil {
		return nil, fmt.Errorf("source queue not found: %w", err)
	}

	// If destination is empty, find the source queue's DLQ source
	// (i.e., the queue that redrives to this DLQ)
	var destQueue QueueRef
	if destinationArn == "" {
		// Find queues whose RedrivePolicy points to the source ARN
		allQueues := mtm.listFn("")
		for _, q := range allQueues {
			rpStr := q.GetRedrivePolicy()
			if rpStr == "" {
				continue
			}
			rp, err := ParseRedrivePolicy(rpStr)
			if err != nil {
				continue
			}
			if rp.DeadLetterTargetArn == sourceArn {
				destQueue = q
				destinationArn = q.GetQueueArn()
				break
			}
		}
		if destQueue == nil {
			return nil, fmt.Errorf("no source queue found for DLQ ARN: %s", sourceArn)
		}
	} else {
		destQueue, err = mtm.lookupFn(destinationArn)
		if err != nil {
			return nil, fmt.Errorf("destination queue not found: %w", err)
		}
	}

	taskHandle := generateTaskHandle()
	task := &MoveTask{
		taskHandle:                   taskHandle,
		sourceArn:                    sourceArn,
		destinationArn:               destinationArn,
		maxNumberOfMessagesPerSecond: maxRate,
		startedAt:                    time.Now().UTC(),
		cancelled:                    make(chan struct{}),
	}
	task.status.Store(MoveTaskStatusRunning)

	mtm.mu.Lock()
	mtm.tasks[taskHandle] = task
	mtm.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		mtm.runTask(ctx, task, sourceQueue, destQueue)
		cancel()
	}()

	return task, nil
}

// CancelTask requests cancellation of a running move task.
// Returns the approximate number of messages moved so far.
func (mtm *MoveTaskManager) CancelTask(taskHandle string) (int, error) {
	mtm.mu.Lock()
	defer mtm.mu.Unlock()

	task, ok := mtm.tasks[taskHandle]
	if !ok {
		return 0, fmt.Errorf("task not found: %s", taskHandle)
	}

	if task.Status() != MoveTaskStatusRunning {
		return int(task.movedMessages.Load()), nil
	}

	// Mark as cancelling and signal the background goroutine.
	// The goroutine will set the final Cancelled status when it stops.
	// sync.Once prevents panic on double-cancel.
	task.status.Store(MoveTaskStatusCancelling)
	task.cancelOnce.Do(func() {
		close(task.cancelled)
	})

	return int(task.movedMessages.Load()), nil
}

// ListTasks returns all move tasks for the given source ARN.
// If sourceArn is empty, returns all tasks.
func (mtm *MoveTaskManager) ListTasks(sourceArn string) []*MoveTask {
	mtm.mu.RLock()
	defer mtm.mu.RUnlock()

	var result []*MoveTask
	for _, t := range mtm.tasks {
		if sourceArn == "" || t.sourceArn == sourceArn {
			result = append(result, t)
		}
	}
	return result
}

// GetTask returns a single move task by its handle.
func (mtm *MoveTaskManager) GetTask(taskHandle string) (*MoveTask, bool) {
	mtm.mu.RLock()
	defer mtm.mu.RUnlock()

	t, ok := mtm.tasks[taskHandle]
	return t, ok
}

// runTask is the background goroutine that moves messages from source to destination.
// It receives messages from the source queue and sends them to the destination,
// optionally rate-limited via a time.Ticker.
func (mtm *MoveTaskManager) runTask(ctx context.Context, task *MoveTask, sourceQueue, destQueue QueueRef) {
	var ticker *time.Ticker
	if task.maxNumberOfMessagesPerSecond > 0 {
		interval := time.Second / time.Duration(task.maxNumberOfMessagesPerSecond)
		ticker = time.NewTicker(interval)
		defer ticker.Stop()
	}

	for {
		select {
		case <-task.cancelled:
			mtm.setTaskStatus(task, MoveTaskStatusCancelled)
			return
		case <-ctx.Done():
			mtm.setTaskStatus(task, MoveTaskStatusCancelled)
			return
		default:
		}

		// Receive up to 10 messages with no long polling
		messages, err := sourceQueue.Store().ReceiveMessages(ctx, 10, 30, 0)
		if err != nil {
			mtm.log.Errorf("move task %s: failed to receive messages from source: %v", task.taskHandle, err)
			mtm.setTaskStatus(task, MoveTaskStatusFailed)
			return
		}

		if len(messages) == 0 {
			// No more messages to move
			mtm.setTaskStatus(task, MoveTaskStatusCompleted)
			return
		}

		for _, msg := range messages {
			select {
			case <-task.cancelled:
				mtm.setTaskStatus(task, MoveTaskStatusCancelled)
				return
			default:
			}

			// Rate limiting
			if ticker != nil {
				<-ticker.C
			}

			// Send to destination first to avoid message loss.
			// If the send fails, the message stays in the source queue.
			store.PrepareForRedrive(msg)

			if err := destQueue.Store().SendMessage(ctx, msg, 0); err != nil {
				// Send failed — message remains in source queue (not deleted)
				mtm.log.Errorf("move task %s: failed to send message to destination: %v", task.taskHandle, err)
				continue
			}

			// Now safe to delete from source
			if err := sourceQueue.Store().DeleteMessage(ctx, msg.ReceiptHandle); err != nil {
				// Delete failed — message may be delivered twice (at-least-once)
				// This is acceptable per SQS semantics, but log for observability.
				mtm.log.Errorf("move task %s: failed to delete message from source (may duplicate): %v", task.taskHandle, err)
			}

			task.movedMessages.Add(1)
		}
	}
}

// setTaskStatus safely updates a task's status.
// When transitioning to a terminal state (completed, cancelled, failed),
// it also records the completion time for TTL-based cleanup.
func (mtm *MoveTaskManager) setTaskStatus(task *MoveTask, status MoveTaskStatus) {
	task.status.Store(status)
	if status == MoveTaskStatusCompleted || status == MoveTaskStatusCancelled || status == MoveTaskStatusFailed {
		task.completedAt.Store(time.Now().UTC())
	}
}

// generateTaskHandle creates a unique task handle.
func generateTaskHandle() string {
	return id.NewHexID()
}
