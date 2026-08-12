// Package store defines the Store interface for pluggable message storage
// backends. Implementations include in-memory, SQLite, and BadgerDB stores.
package store

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// Store defines the interface for a message store backend.
type Store interface {
	// SendMessage adds a message to the queue with an optional delay.
	// Returns an error if the message could not be stored (e.g. database failure).
	SendMessage(ctx context.Context, msg *types.Message, delaySeconds int) error

	// ReceiveMessages retrieves up to maxMessages visible messages from the queue.
	// It blocks for up to waitTimeSeconds if no messages are available (long polling).
	// Returns an error if the store is closed or the context is cancelled.
	ReceiveMessages(ctx context.Context, maxMessages int, visibilityTimeout int, waitTimeSeconds int) ([]*types.Message, error)

	// DeleteMessage removes a message from the queue by its receipt handle.
	// Returns an error if the receipt handle is invalid or the store fails.
	DeleteMessage(ctx context.Context, receiptHandle string) error

	// ChangeMessageVisibility updates the visibility timeout of a message.
	// Returns an error if the receipt handle is invalid or the store fails.
	ChangeMessageVisibility(ctx context.Context, receiptHandle string, visibilityTimeout int) error

	// ApproximateNumberOfMessages returns the approximate number of messages available.
	ApproximateNumberOfMessages() int

	// ApproximateNumberOfMessagesNotVisible returns the approximate number of in-flight messages.
	ApproximateNumberOfMessagesNotVisible() int

	// ApproximateNumberOfMessagesDelayed returns the approximate number of delayed messages.
	ApproximateNumberOfMessagesDelayed() int

	// Purge removes all messages from the queue.
	// Returns an error if the store fails to purge messages.
	Purge(ctx context.Context) error

	// Close releases any resources held by the store.
	// Returns an error if cleanup fails (e.g. database close error).
	Close() error
}

// StoreFactory creates a new Store instance for a queue.
// It receives the queue name, visibility timeout, server secret, and queue attributes
// so the factory can configure FIFO, DLQ, or other store-level behavior.
// Returns an error if the store cannot be created (e.g. database connection failure).
type StoreFactory func(queueName string, visibilityTimeout int, serverSecret []byte, attrs StoreConfig) (Store, error)

// StoreConfig holds store-level configuration derived from queue attributes.
type StoreConfig struct {
	IsFifo                    bool
	ContentBasedDeduplication bool
	MaxReceiveCount           int
	RedriveFunc               RedriveFunc
	Log                       logger.LoggerInterface
	MessageRetentionPeriod    int // seconds; 0 means use default
}

// RedriveFunc is called by a store when a message exceeds maxReceiveCount.
// The store invokes this callback instead of making the message visible again.
type RedriveFunc func(msg *types.Message)

// nowFunc holds the current time provider, used for testability.
// Uses atomic operations for thread-safe swapping in tests.
var nowFunc atomic.Pointer[func() time.Time]

func init() {
	defaultNow := func() time.Time { return time.Now().UTC() }
	nowFunc.Store(&defaultNow)
}

// Now returns the current time, used for testability.
// Thread-safe via atomic pointer; safe for use with t.Parallel().
func Now() time.Time {
	f := nowFunc.Load()
	if f == nil {
		return time.Now().UTC()
	}
	return (*f)()
}

// SetNowFunc replaces the time provider. Intended for testing only.
// Thread-safe via atomic pointer swap.
func SetNowFunc(f func() time.Time) {
	nowFunc.Store(&f)
}

// PrepareForRedrive resets a message's state before sending it to a DLQ.
// This clears the receipt handle, marks the message as visible, and resets
// the approximate receive count so the message gets a fresh start in the DLQ.
// Store implementations should call this before invoking RedriveFunc.
func PrepareForRedrive(msg *types.Message) {
	msg.ReceiptHandle = ""
	msg.IsVisible = true
	msg.ApproximateReceiveCount = 0
}

// GenerateReceiptHandle creates a signed, base64-encoded receipt handle.
// The handle contains the queue name, message ID, receive timestamp, and a
// random nonce, signed with the server secret for tamper resistance.
func GenerateReceiptHandle(queueName, messageID string, now time.Time, serverSecret []byte) (string, error) {
	info := types.ReceiptHandleInfo{
		QueueName:        queueName,
		MessageID:        messageID,
		ReceiveTimestamp: now,
		RandomNonce:      GenerateNonce(),
	}

	data, err := json.Marshal(info)
	if err != nil {
		return "", fmt.Errorf("failed to marshal receipt handle info: %w", err)
	}
	mac := hmac.New(sha256.New, serverSecret)
	mac.Write(data)
	signature := mac.Sum(nil)

	handle := map[string]string{
		"data":      base64.StdEncoding.EncodeToString(data),
		"signature": hex.EncodeToString(signature),
	}

	encoded, err := json.Marshal(handle)
	if err != nil {
		return "", fmt.Errorf("failed to marshal receipt handle: %w", err)
	}
	return base64.StdEncoding.EncodeToString(encoded), nil
}

// GenerateNonce creates a random hex string.
// Falls back to a time-based nonce if crypto/rand fails.
func GenerateNonce() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based nonce if crypto/rand fails
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// ComputeContentBasedDedupID generates a deduplication ID from the message body using SHA-256.
func ComputeContentBasedDedupID(body string) string {
	h := sha256.Sum256([]byte(body))
	return hex.EncodeToString(h[:])
}

// CleanExpiredDedupEntries removes dedup cache entries that have exceeded the dedup window.
func CleanExpiredDedupEntries(cache map[string]time.Time, now time.Time) {
	for id, expiry := range cache {
		if now.After(expiry) {
			delete(cache, id)
		}
	}
}
