package dlq

import (
	"context"
	"fmt"
	"sync"
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
	MovedMessages                int
	StartedAt                    time.Time
	Cancelled                    chan struct{}
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
		Cancelled:                    make(chan struct{}),
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
		return task.MovedMessages, nil
	}

	task.Status = MoveTaskStatusCancelling
	close(task.Cancelled)
	task.Status = MoveTaskStatusCancelled

	return task.MovedMessages, nil
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
		case <-task.Cancelled:
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
			case <-task.Cancelled:
				mtm.setTaskStatus(task, MoveTaskStatusCancelled)
				return
			default:
			}

			// Rate limiting
			if ticker != nil {
				<-ticker.C
			}

			// Delete from source first (using the receipt handle from ReceiveMessages)
			if err := sourceQueue.Store().DeleteMessage(ctx, msg.ReceiptHandle); err != nil {
				// If we can't delete, skip this message
				continue
			}

			// Reset message state for redelivery (same as redriveMessage)
			msg.ReceiptHandle = ""
			msg.IsVisible = true
			msg.ApproximateReceiveCount = 0

			if err := destQueue.Store().SendMessage(ctx, msg, 0); err != nil {
				// Log and continue — partial failures are acceptable
				continue
			}

			mtm.mu.Lock()
			task.MovedMessages++
			mtm.mu.Unlock()
		}
	}
}

// setTaskStatus safely updates a task's status.
func (mtm *MoveTaskManager) setTaskStatus(task *MoveTask, status MoveTaskStatus) {
	mtm.mu.Lock()
	task.Status = status
	mtm.mu.Unlock()
}

// generateTaskHandle creates a unique task handle.
func generateTaskHandle() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
