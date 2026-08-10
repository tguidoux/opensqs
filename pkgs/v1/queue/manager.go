package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/dlq"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// QueueManager manages the lifecycle of SQS queues.
type QueueManager struct {
	mu           sync.RWMutex
	queues       map[string]*Queue
	nodeAddress  string
	accountID    string
	region       string
	serverSecret []byte
	storeFactory store.StoreFactory
}

// NewQueueManager creates a new QueueManager.
// The storeFactory parameter determines which Store implementation is used for each queue.
func NewQueueManager(nodeAddress, accountID, region string, serverSecret []byte, storeFactory store.StoreFactory) *QueueManager {
	return &QueueManager{
		queues:       make(map[string]*Queue),
		nodeAddress:  nodeAddress,
		accountID:    accountID,
		region:       region,
		serverSecret: serverSecret,
		storeFactory: storeFactory,
	}
}

// CreateQueue creates a new queue with the given name and attributes.
// Returns an error if a queue with the same name already exists.
func (qm *QueueManager) CreateQueue(name string, attrs *QueueAttributes) (*Queue, error) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if existing, ok := qm.queues[name]; ok {
		// Check if attributes match
		if attrs != nil && !attributesMatch(existing.Attributes(), attrs) {
			return nil, NewQueueNameExists("")
		}
		return existing, nil
	}

	if attrs == nil {
		attrs = NewDefaultQueueAttributes()
	}

	// Set the queue ARN
	attrs.QueueArn = buildQueueARN(qm.region, qm.accountID, name)

	// Parse RedrivePolicy to configure dead-letter queue settings
	storeCfg := store.StoreConfig{
		IsFifo:                    attrs.FifoQueue,
		ContentBasedDeduplication: attrs.ContentBasedDeduplication,
	}

	if attrs.RedrivePolicy != "" {
		rp, err := dlq.ParseRedrivePolicy(attrs.RedrivePolicy)
		if err != nil {
			return nil, fmt.Errorf("invalid redrive policy for queue %q: %w", name, err)
		}
		if rp.MaxReceiveCount > 0 {
			storeCfg.MaxReceiveCount = rp.MaxReceiveCount
			// Set up the redrive callback that sends messages to the DLQ
			dlqArn := rp.DeadLetterTargetArn
			storeCfg.RedriveFunc = func(msg *types.Message) {
				qm.redriveMessage(dlqArn, msg)
			}
		}
	}

	// Create the message store via the factory
	msgStore, err := qm.storeFactory(name, attrs.VisibilityTimeout, qm.serverSecret, storeCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create store for queue %q: %w", name, err)
	}

	q := NewQueue(name, attrs, msgStore)
	qm.queues[name] = q

	return q, nil
}

// DeleteQueue removes a queue and all its messages.
func (qm *QueueManager) DeleteQueue(name string) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	q, ok := qm.queues[name]
	if !ok {
		return NewQueueDoesNotExist(fmt.Sprintf("The specified queue does not exist: %s", name))
	}

	if err := q.Store().Close(); err != nil {
		return fmt.Errorf("failed to close store for queue %q: %w", name, err)
	}

	delete(qm.queues, name)

	return nil
}

// LookupQueue returns the queue with the given name.
func (qm *QueueManager) LookupQueue(name string) (*Queue, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	q, ok := qm.queues[name]
	if !ok {
		return nil, NewQueueDoesNotExist(fmt.Sprintf("The specified queue does not exist: %s", name))
	}

	return q, nil
}

// LookupQueueByURL extracts the queue name from a URL and looks it up.
func (qm *QueueManager) LookupQueueByURL(queueURL string) (*Queue, error) {
	name := ExtractQueueNameFromURL(queueURL)
	if name == "" {
		return nil, NewInvalidParameterValue(fmt.Sprintf("Invalid queue URL: %s", queueURL))
	}
	return qm.LookupQueue(name)
}

// ListQueues returns all queue names, optionally filtered by a prefix.
func (qm *QueueManager) ListQueues(prefix string) []*Queue {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	var result []*Queue
	for _, q := range qm.queues {
		if prefix == "" || strings.HasPrefix(q.Name(), prefix) {
			result = append(result, q)
		}
	}
	return result
}

// ListQueueURLs returns all queue URLs, optionally filtered by a prefix.
func (qm *QueueManager) ListQueueURLs(prefix string) []string {
	queues := qm.ListQueues(prefix)
	urls := make([]string, 0, len(queues))
	for _, q := range queues {
		urls = append(urls, q.URL(qm.nodeAddress, qm.accountID))
	}
	return urls
}

// PurgeQueue removes all messages from a queue.
func (qm *QueueManager) PurgeQueue(ctx context.Context, name string) error {
	q, err := qm.LookupQueue(name)
	if err != nil {
		return err
	}

	return q.Store().Purge(ctx)
}

// QueueURL returns the URL for a queue name.
func (qm *QueueManager) QueueURL(name string) string {
	q, err := qm.LookupQueue(name)
	if err != nil {
		return fmt.Sprintf("http://%s/%s/%s", qm.nodeAddress, qm.accountID, name)
	}
	return q.URL(qm.nodeAddress, qm.accountID)
}

// NodeAddress returns the configured node address.
func (qm *QueueManager) NodeAddress() string {
	return qm.nodeAddress
}

// LookupQueueByArn looks up a queue by its ARN.
// ARN format: arn:aws:sqs:<region>:<accountId>:<queueName>
func (qm *QueueManager) LookupQueueByArn(arn string) (*Queue, error) {
	// Extract queue name from the ARN (last segment after the last colon)
	idx := strings.LastIndex(arn, ":")
	if idx >= 0 {
		return qm.LookupQueue(arn[idx+1:])
	}
	return nil, NewQueueDoesNotExist(fmt.Sprintf("Invalid ARN or queue does not exist: %s", arn))
}

// redriveMessage sends a message to the dead-letter queue identified by the given ARN.
// Errors are logged but not returned to match SQS behavior (redrive is best-effort).
func (qm *QueueManager) redriveMessage(dlqArn string, msg *types.Message) {
	dlqQueue, err := qm.LookupQueueByArn(dlqArn)
	if err != nil {
		// DLQ doesn't exist — cannot redrive, message is lost
		// This matches AWS SQS behavior where a misconfigured DLQ silently drops messages
		log.Printf("redrive: DLQ %s not found, message %s will be lost: %v", dlqArn, msg.MessageID, err)
		return
	}

	// Reset message state for redelivery
	store.PrepareForRedrive(msg)

	// Send to the DLQ with no delay
	if err := dlqQueue.Store().SendMessage(context.Background(), msg, 0); err != nil {
		log.Printf("redrive: failed to send message %s to DLQ %s: %v", msg.MessageID, dlqArn, err)
	}
}

// AccountID returns the configured account ID.
func (qm *QueueManager) AccountID() string {
	return qm.accountID
}

// Region returns the configured region.
func (qm *QueueManager) Region() string {
	return qm.region
}

// Shutdown gracefully shuts down all queues by closing their stores.
// It should be called after the HTTP server has stopped accepting new requests.
// The context allows callers to set a deadline for the shutdown.
// The write lock is released during Close() calls to avoid blocking for slow stores.
func (qm *QueueManager) Shutdown(ctx context.Context) error {
	// Snapshot the queues under the lock, then release it so Close() calls
	// don't block other operations and respect the context deadline.
	qm.mu.Lock()
	queues := make(map[string]*Queue, len(qm.queues))
	for name, q := range qm.queues {
		queues[name] = q
	}
	qm.queues = make(map[string]*Queue)
	qm.mu.Unlock()

	var errs []error
	for name, q := range queues {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("shutdown interrupted: %w", errors.Join(errs...))
		}
		if err := q.Store().Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close store for queue %s: %w", name, err))
		}
	}

	return errors.Join(errs...)
}

// attributesMatch checks if two QueueAttributes are equivalent for create-if-exists semantics.
// Compares all user-settable attributes using their string representations to ensure
// thread-safe reads without holding the mutex.
func attributesMatch(a, b *QueueAttributes) bool {
	aAttrs := a.AllAttributes()
	bAttrs := b.AllAttributes()
	// QueueArn is server-assigned, not client-provided, so exclude it from comparison
	delete(aAttrs, types.AttributeQueueArn)
	delete(bAttrs, types.AttributeQueueArn)
	if len(aAttrs) != len(bAttrs) {
		return false
	}
	for k, v := range aAttrs {
		if bv, ok := bAttrs[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// ExtractQueueNameFromURL extracts the queue name from a queue URL.
// URLs are in the format: http://host/accountId/queueName
// It strips query strings and trailing slashes, then returns the last path segment.
func ExtractQueueNameFromURL(queueURL string) string {
	if queueURL == "" {
		return ""
	}
	// Strip query string
	if idx := strings.Index(queueURL, "?"); idx >= 0 {
		queueURL = queueURL[:idx]
	}
	// Strip trailing slash
	queueURL = strings.TrimSuffix(queueURL, "/")
	// Find the last segment after the last /
	for i := len(queueURL) - 1; i >= 0; i-- {
		if queueURL[i] == '/' {
			return queueURL[i+1:]
		}
	}
	return queueURL
}

// buildQueueARN constructs an ARN for a queue.
// Format: arn:aws:sqs:<region>:<accountId>:<queueName>
func buildQueueARN(region, accountID, name string) string {
	return fmt.Sprintf("arn:aws:sqs:%s:%s:%s", region, accountID, name)
}
