package queue

import (
	"fmt"
	"strings"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// Queue represents an SQS queue with its attributes and message store.
type Queue struct {
	name       string
	attributes *QueueAttributes
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
	out := make(map[string]string, len(q.tags))
	for k, v := range q.tags {
		out[k] = v
	}
	return out
}

// SetTags sets the queue tags. The tags map is copied to avoid external mutation.
func (q *Queue) SetTags(tags map[string]string) {
	q.tags = make(map[string]string, len(tags))
	for k, v := range tags {
		q.tags[k] = v
	}
}

// IsFifo returns true if this is a FIFO queue.
func (q *Queue) IsFifo() bool {
	return q.attributes.FifoQueue
}

// GetQueueArn returns the queue ARN from attributes.
func (q *Queue) GetQueueArn() string {
	return q.attributes.QueueArn
}

// GetRedrivePolicy returns the redrive policy from attributes.
func (q *Queue) GetRedrivePolicy() string {
	return q.attributes.RedrivePolicy
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
	return fmt.Sprintf("%s://%s/%s/%s", scheme, nodeAddress, accountID, q.name)
}

// ARN returns the ARN for this queue.
func (q *Queue) ARN(region, accountID string) string {
	return fmt.Sprintf("arn:aws:sqs:%s:%s:%s", region, accountID, q.name)
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
