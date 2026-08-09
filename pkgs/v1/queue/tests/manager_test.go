package queue_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func newTestManager() *queue.QueueManager {
	factory := func(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) (store.Store, error) {
		return memory.NewMemoryStore(queueName, visibilityTimeout, serverSecret, cfg), nil
	}
	return queue.NewQueueManager("localhost:9324", "000000000000", "us-east-1", []byte("test-secret"), factory)
}

func TestCreateQueue(t *testing.T) {
	qm := newTestManager()

	q, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)
	assert.Equal(t, "test-queue", q.Name())
	assert.Equal(t, types.DefaultVisibilityTimeout, q.Attributes().VisibilityTimeout)
}

func TestCreateQueueDuplicate(t *testing.T) {
	qm := newTestManager()

	_, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)

	// Creating with same (default) attributes should return the existing queue
	q, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)
	assert.Equal(t, "test-queue", q.Name())
}

func TestCreateQueueDuplicateDifferentAttrs(t *testing.T) {
	qm := newTestManager()

	_, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)

	// Creating with different attributes should fail
	attrs := queue.NewDefaultQueueAttributes()
	attrs.VisibilityTimeout = 60
	_, err = qm.CreateQueue("test-queue", attrs)
	assert.Error(t, err)
}

func TestDeleteQueue(t *testing.T) {
	qm := newTestManager()

	_, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)

	err = qm.DeleteQueue("test-queue")
	require.NoError(t, err)

	_, err = qm.LookupQueue("test-queue")
	assert.Error(t, err)
}

func TestDeleteQueueNotFound(t *testing.T) {
	qm := newTestManager()

	err := qm.DeleteQueue("nonexistent")
	assert.Error(t, err)
}

func TestLookupQueue(t *testing.T) {
	qm := newTestManager()

	_, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)

	q, err := qm.LookupQueue("test-queue")
	require.NoError(t, err)
	assert.Equal(t, "test-queue", q.Name())
}

func TestLookupQueueNotFound(t *testing.T) {
	qm := newTestManager()

	_, err := qm.LookupQueue("nonexistent")
	assert.Error(t, err)
}

func TestLookupQueueByURL(t *testing.T) {
	qm := newTestManager()

	_, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)

	url := qm.QueueURL("test-queue")
	q, err := qm.LookupQueueByURL(url)
	require.NoError(t, err)
	assert.Equal(t, "test-queue", q.Name())
}

func TestListQueues(t *testing.T) {
	qm := newTestManager()

	_, _ = qm.CreateQueue("queue-a", nil)
	_, _ = qm.CreateQueue("queue-b", nil)
	_, _ = qm.CreateQueue("other-queue", nil)

	queues := qm.ListQueues("")
	assert.Len(t, queues, 3)
}

func TestListQueuesWithPrefix(t *testing.T) {
	qm := newTestManager()

	_, _ = qm.CreateQueue("queue-a", nil)
	_, _ = qm.CreateQueue("queue-b", nil)
	_, _ = qm.CreateQueue("other-queue", nil)

	queues := qm.ListQueues("queue-")
	assert.Len(t, queues, 2)
}

func TestListQueueURLs(t *testing.T) {
	qm := newTestManager()

	_, _ = qm.CreateQueue("test-queue", nil)

	urls := qm.ListQueueURLs("")
	assert.Len(t, urls, 1)
	assert.Contains(t, urls[0], "test-queue")
	assert.Contains(t, urls[0], "localhost:9324")
}

func TestQueueURL(t *testing.T) {
	qm := newTestManager()

	url := qm.QueueURL("my-queue")
	assert.Equal(t, "http://localhost:9324/000000000000/my-queue", url)
}

func TestQueueURLMethod(t *testing.T) {
	qm := newTestManager()

	q, err := qm.CreateQueue("my-queue", nil)
	require.NoError(t, err)

	url := q.URL("localhost:9324", "000000000000")
	assert.Equal(t, "http://localhost:9324/000000000000/my-queue", url)
}

func TestQueueARN(t *testing.T) {
	qm := newTestManager()

	q, err := qm.CreateQueue("my-queue", nil)
	require.NoError(t, err)

	arn := q.ARN("us-east-1", "000000000000")
	assert.Equal(t, "arn:aws:sqs:us-east-1:000000000000:my-queue", arn)
}

func TestQueueIsFifo(t *testing.T) {
	qm := newTestManager()

	attrs := queue.NewDefaultQueueAttributes()
	attrs.FifoQueue = true
	q, err := qm.CreateQueue("my-queue.fifo", attrs)
	require.NoError(t, err)

	assert.True(t, q.IsFifo())
}

func TestQueueGetAttributeComputed(t *testing.T) {
	qm := newTestManager()

	q, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)

	v, ok := q.GetAttribute(types.AttributeApproximateNumberOfMessages)
	assert.True(t, ok)
	assert.Equal(t, "0", v)

	v, ok = q.GetAttribute(types.AttributeApproximateNumberOfMessagesNotVisible)
	assert.True(t, ok)
	assert.Equal(t, "0", v)

	v, ok = q.GetAttribute(types.AttributeApproximateNumberOfMessagesDelayed)
	assert.True(t, ok)
	assert.Equal(t, "0", v)
}

func TestQueueGetAttributeRegular(t *testing.T) {
	qm := newTestManager()

	q, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)

	v, ok := q.GetAttribute(types.AttributeVisibilityTimeout)
	assert.True(t, ok)
	assert.Equal(t, "30", v)
}

func TestPurgeQueue(t *testing.T) {
	qm := newTestManager()

	q, err := qm.CreateQueue("test-queue", nil)
	require.NoError(t, err)
	require.NotNil(t, q)

	// Purge should work on empty queue
	err = qm.PurgeQueue(context.Background(), "test-queue")
	require.NoError(t, err)
}

func TestPurgeQueueNotFound(t *testing.T) {
	qm := newTestManager()

	err := qm.PurgeQueue(context.Background(), "nonexistent")
	assert.Error(t, err)
}
