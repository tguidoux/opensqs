// Package queue provides the core SQS queue engine, including queue management,
// message operations, visibility timeouts, and pluggable storage backends.
package queue

import (
	"fmt"
	"maps"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// Queue represents an SQS queue with its attributes and message store.
type Queue struct {
	name       string
	attributes *QueueAttributes
	tagsMu     sync.RWMutex
	tags       map[string]string
	store      store.Store
}

// NewQueue creates a new Queue instance with the given name and default attributes.
func NewQueue(name string, attrs *QueueAttributes, messageStore store.Store) *Queue {
	if attrs == nil {
		attrs = NewDefaultQueueAttributes()
	}

	return &Queue{
		name:       name,
		attributes: attrs,
		tags:       make(map[string]string),
		store:      messageStore,
	}
}

// Name returns the queue name.
func (q *Queue) Name() string {
	return q.name
}

// Attributes returns the queue attributes.
func (q *Queue) Attributes() *QueueAttributes {
	return q.attributes
}

// Store returns the message store for this queue.
func (q *Queue) Store() store.Store {
	return q.store
}

// Tags returns a copy of the queue tags to prevent external mutation.
func (q *Queue) Tags() map[string]string {
	q.tagsMu.RLock()
	defer q.tagsMu.RUnlock()
	return maps.Clone(q.tags)
}

// SetTags sets the queue tags. The tags map is copied to avoid external mutation.
func (q *Queue) SetTags(tags map[string]string) {
	q.tagsMu.Lock()
	defer q.tagsMu.Unlock()
	q.tags = maps.Clone(tags)
}

// UpdateTags atomically applies a mutation function to the queue tags.
// The fn receives a copy of the current tags and returns the updated tags.
// This prevents the get-modify-set race that would occur if callers used
// Tags() + SetTags() separately.
func (q *Queue) UpdateTags(fn func(tags map[string]string) map[string]string) {
	q.tagsMu.Lock()
	defer q.tagsMu.Unlock()
	q.tags = fn(maps.Clone(q.tags))
}

// IsFifo returns true if this is a FIFO queue.
func (q *Queue) IsFifo() bool {
	val, _ := q.attributes.GetAttribute(types.AttributeFifoQueue)
	b, _ := strconv.ParseBool(val)
	return b
}

// GetQueueArn returns the queue ARN from attributes.
func (q *Queue) GetQueueArn() string {
	return q.attributes.GetQueueArn()
}

// GetRedrivePolicy returns the redrive policy from attributes.
func (q *Queue) GetRedrivePolicy() string {
	return q.attributes.GetRedrivePolicy()
}

// URL returns the full URL for this queue.
// If nodeAddress includes a scheme (e.g. "https://"), it is used;
// otherwise "http://" is assumed.
func (q *Queue) URL(nodeAddress, accountID string) string {
	scheme := "http"
	if strings.HasPrefix(nodeAddress, "https://") {
		scheme = "https"
		nodeAddress = strings.TrimPrefix(nodeAddress, "https://")
	} else {
		nodeAddress = strings.TrimPrefix(nodeAddress, "http://")
	}
	u := &url.URL{
		Scheme: scheme,
		Host:   nodeAddress,
		Path:   fmt.Sprintf("%s/%s", accountID, q.name),
	}
	return u.String()
}

// ARN returns the ARN for this queue.
func (q *Queue) ARN(region, accountID string) string {
	return buildQueueARN(region, accountID, q.name)
}

// ApproximateNumberOfMessages returns the approximate count of available messages.
func (q *Queue) ApproximateNumberOfMessages() int {
	return q.store.ApproximateNumberOfMessages()
}

// ApproximateNumberOfMessagesNotVisible returns the approximate count of in-flight messages.
func (q *Queue) ApproximateNumberOfMessagesNotVisible() int {
	return q.store.ApproximateNumberOfMessagesNotVisible()
}

// ApproximateNumberOfMessagesDelayed returns the approximate count of delayed messages.
func (q *Queue) ApproximateNumberOfMessagesDelayed() int {
	return q.store.ApproximateNumberOfMessagesDelayed()
}

// GetAttribute returns a specific attribute or a computed one.
func (q *Queue) GetAttribute(name string) (string, bool) {
	switch name {
	case types.AttributeApproximateNumberOfMessages:
		return fmt.Sprintf("%d", q.ApproximateNumberOfMessages()), true
	case types.AttributeApproximateNumberOfMessagesNotVisible:
		return fmt.Sprintf("%d", q.ApproximateNumberOfMessagesNotVisible()), true
	case types.AttributeApproximateNumberOfMessagesDelayed:
		return fmt.Sprintf("%d", q.ApproximateNumberOfMessagesDelayed()), true
	default:
		return q.attributes.GetAttribute(name)
	}
}
