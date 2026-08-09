package dlq

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

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
type MoveTask struct {
	TaskHandle                   string
	SourceArn                    string
	DestinationArn               string
	Status                       MoveTaskStatus
	MaxNumberOfMessagesPerSecond int
	movedMessages                atomic.Int64
	StartedAt                    time.Time
	cancelled                    chan struct{}
}

// MovedMessages returns the number of messages moved so far.
// This method is thread-safe.
func (t *MoveTask) MovedMessages() int {
	return int(t.movedMessages.Load())
}

// MoveTaskManager manages the lifecycle of message move tasks.
// It tracks active and completed tasks, starts background goroutines
// to move messages, and supports cancellation.
type MoveTaskManager struct {
	mu       sync.RWMutex
	tasks    map[string]*MoveTask
	lookupFn QueueLookupFunc
	listFn   QueueListFunc
}

// NewMoveTaskManager creates a new MoveTaskManager wired to the given lookup and list functions.
// The lookup function resolves a queue by ARN, the list function lists queues by prefix.
// Both are typically wired from *queue.QueueManager.
func NewMoveTaskManager(lookupFn QueueLookupFunc, listFn QueueListFunc) *MoveTaskManager {
	return &MoveTaskManager{
		tasks:    make(map[string]*MoveTask),
		lookupFn: lookupFn,
		listFn:   listFn,
	}
}

// StartTask creates and starts a new message move task.
// If destinationArn is empty, messages are moved back to the source queue
// that originally sent them (the DLQ's source queue).
// If maxRate is > 0, message movement is rate-limited to that many messages per second.
func (mtm *MoveTaskManager) StartTask(sourceArn, destinationArn string, maxRate int) (*MoveTask, error) {
	if sourceArn == "" {
		return nil, fmt.Errorf("source ARN is required")
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
		TaskHandle:                   taskHandle,
		SourceArn:                    sourceArn,
		DestinationArn:               destinationArn,
		Status:                       MoveTaskStatusRunning,
		MaxNumberOfMessagesPerSecond: maxRate,
		StartedAt:                    time.Now().UTC(),
		cancelled:                    make(chan struct{}),
	}

	mtm.mu.Lock()
	mtm.tasks[taskHandle] = task
	mtm.mu.Unlock()

	go mtm.runTask(context.Background(), task, sourceQueue, destQueue)

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

	if task.Status != MoveTaskStatusRunning {
		return int(task.movedMessages.Load()), nil
	}

	// Mark as cancelling and signal the background goroutine.
	// The goroutine will set the final Cancelled status when it stops.
	task.Status = MoveTaskStatusCancelling
	close(task.cancelled)

	return int(task.movedMessages.Load()), nil
}

// ListTasks returns all move tasks for the given source ARN.
// If sourceArn is empty, returns all tasks.
func (mtm *MoveTaskManager) ListTasks(sourceArn string) []*MoveTask {
	mtm.mu.RLock()
	defer mtm.mu.RUnlock()

	var result []*MoveTask
	for _, t := range mtm.tasks {
		if sourceArn == "" || t.SourceArn == sourceArn {
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
	if task.MaxNumberOfMessagesPerSecond > 0 {
		interval := time.Second / time.Duration(task.MaxNumberOfMessagesPerSecond)
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
			msg.ReceiptHandle = ""
			msg.IsVisible = true
			msg.ApproximateReceiveCount = 0

			if err := destQueue.Store().SendMessage(ctx, msg, 0); err != nil {
				// Send failed — message remains in source queue (not deleted)
				continue
			}

			// Now safe to delete from source
			if err := sourceQueue.Store().DeleteMessage(ctx, msg.ReceiptHandle); err != nil {
				// Delete failed — message may be delivered twice (at-least-once)
				// This is acceptable per SQS semantics
			}

			task.movedMessages.Add(1)
		}
	}
}

// setTaskStatus safely updates a task's status.
func (mtm *MoveTaskManager) setTaskStatus(task *MoveTask, status MoveTaskStatus) {
	mtm.mu.Lock()
	task.Status = status
	mtm.mu.Unlock()
}

// generateTaskHandle creates a unique task handle using crypto/rand.
func generateTaskHandle() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp if crypto/rand fails
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
