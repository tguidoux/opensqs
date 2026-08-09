package store

import (
	"context"
	"time"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// Store defines the interface for a message store backend.
type Store interface {
	// SendMessage adds a message to the queue with an optional delay.
	SendMessage(ctx context.Context, msg *types.Message, delaySeconds int) error

	// ReceiveMessages retrieves up to maxMessages visible messages from the queue.
	// It blocks for up to waitTimeSeconds if no messages are available (long polling).
	ReceiveMessages(ctx context.Context, maxMessages int, visibilityTimeout int, waitTimeSeconds int) ([]*types.Message, error)

	// DeleteMessage removes a message from the queue by its receipt handle.
	DeleteMessage(ctx context.Context, receiptHandle string) error

	// ChangeMessageVisibility updates the visibility timeout of a message.
	ChangeMessageVisibility(ctx context.Context, receiptHandle string, visibilityTimeout int) error

	// ApproximateNumberOfMessages returns the approximate number of messages available.
	ApproximateNumberOfMessages() int

	// ApproximateNumberOfMessagesNotVisible returns the approximate number of in-flight messages.
	ApproximateNumberOfMessagesNotVisible() int

	// ApproximateNumberOfMessagesDelayed returns the approximate number of delayed messages.
	ApproximateNumberOfMessagesDelayed() int

	// Purge removes all messages from the queue.
	Purge(ctx context.Context) error

	// Close releases any resources held by the store.
	Close() error
}

// MessageCounts holds the approximate counts for a queue.
type MessageCounts struct {
	Available int
	InFlight  int
	Delayed   int
}

// StoreFactory creates a new Store instance for a queue.
// It receives the queue name, visibility timeout, server secret, and queue attributes
// so the factory can configure FIFO, DLQ, or other store-level behavior.
type StoreFactory func(queueName string, visibilityTimeout int, serverSecret []byte, attrs StoreConfig) Store

// StoreConfig holds store-level configuration derived from queue attributes.
type StoreConfig struct {
	IsFifo                    bool
	ContentBasedDeduplication bool
	MaxReceiveCount           int
	RedriveFunc               RedriveFunc
}

// RedriveFunc is called by a store when a message exceeds maxReceiveCount.
// The store invokes this callback instead of making the message visible again.
type RedriveFunc func(msg *types.Message)

// Now returns the current time, used for testability.
var Now = func() time.Time { return time.Now().UTC() }
