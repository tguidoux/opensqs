package queue

import (
	"fmt"

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

// Tags returns the queue tags.
func (q *Queue) Tags() map[string]string {
	return q.tags
}

// SetTags sets the queue tags.
func (q *Queue) SetTags(tags map[string]string) {
	q.tags = tags
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
func (q *Queue) URL(nodeAddress, accountID string) string {
	return fmt.Sprintf("http://%s/%s/%s", nodeAddress, accountID, q.name)
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
