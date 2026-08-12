package queue_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func TestNewDefaultQueueAttributes(t *testing.T) {
	attrs := queue.NewDefaultQueueAttributes()

	assert.Equal(t, types.DefaultVisibilityTimeout, attrs.VisibilityTimeout)
	assert.Equal(t, types.DefaultDelaySeconds, attrs.DelaySeconds)
	assert.Equal(t, types.DefaultMaximumMessageSize, attrs.MaximumMessageSize)
	assert.Equal(t, types.DefaultMessageRetentionPeriod, attrs.MessageRetentionPeriod)
	assert.Equal(t, types.DefaultReceiveMessageWaitTime, attrs.ReceiveMessageWaitTimeSeconds)
	assert.True(t, attrs.SqsManagedSseEnabled)
}

func TestGetAttribute(t *testing.T) {
	attrs := queue.NewDefaultQueueAttributes()

	v, ok := attrs.GetAttribute(types.AttributeVisibilityTimeout)
	assert.True(t, ok)
	assert.Equal(t, "30", v)

	v, ok = attrs.GetAttribute(types.AttributeDelaySeconds)
	assert.True(t, ok)
	assert.Equal(t, "0", v)

	_, ok = attrs.GetAttribute("NonExistent")
	assert.False(t, ok)
}

func TestSetAttribute(t *testing.T) {
	attrs := queue.NewDefaultQueueAttributes()

	err := attrs.SetAttribute(types.AttributeVisibilityTimeout, "60")
	require.NoError(t, err)
	assert.Equal(t, 60, attrs.VisibilityTimeout)

	err = attrs.SetAttribute(types.AttributeFifoQueue, "true")
	require.NoError(t, err)
	assert.True(t, attrs.FifoQueue)

	err = attrs.SetAttribute(types.AttributeVisibilityTimeout, "invalid")
	assert.Error(t, err)

	err = attrs.SetAttribute("UnknownAttribute", "value")
	assert.Error(t, err)
}

func TestSetAttributesAtomic(t *testing.T) {
	attrs := queue.NewDefaultQueueAttributes()

	err := attrs.SetAttributes(map[string]string{
		types.AttributeVisibilityTimeout: "60",
		types.AttributeRedrivePolicy:     "{bad json",
	})
	require.Error(t, err)

	v, ok := attrs.GetAttribute(types.AttributeVisibilityTimeout)
	assert.True(t, ok)
	assert.Equal(t, "30", v)

	redrive, ok := attrs.GetAttribute(types.AttributeRedrivePolicy)
	assert.True(t, ok)
	assert.Empty(t, redrive)
}

func TestAllAttributes(t *testing.T) {
	attrs := queue.NewDefaultQueueAttributes()
	all := attrs.AllAttributes()

	assert.Contains(t, all, types.AttributeVisibilityTimeout)
	assert.Contains(t, all, types.AttributeDelaySeconds)
	assert.Contains(t, all, types.AttributeMaximumMessageSize)
	assert.Contains(t, all, types.AttributeMessageRetentionPeriod)
	assert.Equal(t, "30", all[types.AttributeVisibilityTimeout])
}

func TestSQSError(t *testing.T) {
	err := queue.NewInvalidAction("bad action")
	assert.Equal(t, "InvalidAction", err.Code())
	assert.Equal(t, 400, err.HTTPStatusCode())
	assert.Equal(t, "Sender", err.ErrorType())
	assert.Equal(t, "bad action", err.Message())
	assert.Contains(t, err.Error(), "InvalidAction")
}

func TestQueueDoesNotExist(t *testing.T) {
	err := queue.NewQueueDoesNotExist("")
	assert.Equal(t, "AWS.SimpleQueueService.NonExistentQueue", err.Code())
	assert.Equal(t, 400, err.HTTPStatusCode())
	assert.Contains(t, err.Message(), "does not exist")
}

func TestInvalidParameterValue(t *testing.T) {
	err := queue.NewInvalidParameterValue("bad value")
	assert.Equal(t, "InvalidParameterValue", err.Code())
	assert.Equal(t, "bad value", err.Message())
}

func TestQueueNameExists(t *testing.T) {
	err := queue.NewQueueNameExists("")
	assert.Equal(t, "QueueAlreadyExists", err.Code())
	assert.Contains(t, err.Message(), "already exists")
}

func TestInternalError(t *testing.T) {
	err := queue.NewInternalError("")
	assert.Equal(t, "InternalError", err.Code())
	assert.Equal(t, 500, err.HTTPStatusCode())
	assert.Equal(t, "Receiver", err.ErrorType())
}

func TestLimitsVerifyMessageSize(t *testing.T) {
	limits := queue.NewLimits(queue.StrictMode)

	err := limits.VerifyMessageSize("hello", 262144)
	assert.NoError(t, err)

	err = limits.VerifyMessageSize(string(make([]byte, 262145)), 262144)
	assert.Error(t, err)
}

func TestLimitsVerifyBatchSize(t *testing.T) {
	limits := queue.NewLimits(queue.StrictMode)

	err := limits.VerifyBatchSize(5)
	assert.NoError(t, err)

	err = limits.VerifyBatchSize(11)
	assert.Error(t, err)
}

func TestLimitsVerifyVisibilityTimeout(t *testing.T) {
	limits := queue.NewLimits(queue.StrictMode)

	err := limits.VerifyVisibilityTimeout(30)
	assert.NoError(t, err)

	err = limits.VerifyVisibilityTimeout(-1)
	assert.Error(t, err)

	err = limits.VerifyVisibilityTimeout(50000)
	assert.Error(t, err)
}

func TestLimitsVerifyDelaySeconds(t *testing.T) {
	limits := queue.NewLimits(queue.StrictMode)

	err := limits.VerifyDelaySeconds(0)
	assert.NoError(t, err)

	err = limits.VerifyDelaySeconds(1000)
	assert.Error(t, err)
}

func TestLimitsVerifyReceiveMessageWaitTime(t *testing.T) {
	limits := queue.NewLimits(queue.StrictMode)

	err := limits.VerifyReceiveMessageWaitTime(10)
	assert.NoError(t, err)

	err = limits.VerifyReceiveMessageWaitTime(25)
	assert.Error(t, err)
}

func TestLimitsVerifyMaxNumberOfMessages(t *testing.T) {
	limits := queue.NewLimits(queue.StrictMode)

	err := limits.VerifyMaxNumberOfMessages(5)
	assert.NoError(t, err)

	err = limits.VerifyMaxNumberOfMessages(0)
	assert.Error(t, err)

	err = limits.VerifyMaxNumberOfMessages(11)
	assert.Error(t, err)
}

func TestLimitsVerifyQueueName(t *testing.T) {
	limits := queue.NewLimits(queue.StrictMode)

	err := limits.VerifyQueueName("my-queue")
	assert.NoError(t, err)

	err = limits.VerifyQueueName("my_queue_123")
	assert.NoError(t, err)

	err = limits.VerifyQueueName("my-queue.fifo")
	assert.NoError(t, err)

	err = limits.VerifyQueueName("")
	assert.Error(t, err)

	err = limits.VerifyQueueName("queue with spaces")
	assert.Error(t, err)

	err = limits.VerifyQueueName("queue@special")
	assert.Error(t, err)
}

func TestLimitsVerifyMessageRetentionPeriod(t *testing.T) {
	limits := queue.NewLimits(queue.StrictMode)

	err := limits.VerifyMessageRetentionPeriod(345600)
	assert.NoError(t, err)

	err = limits.VerifyMessageRetentionPeriod(30)
	assert.Error(t, err)

	err = limits.VerifyMessageRetentionPeriod(2000000)
	assert.Error(t, err)
}

func TestLimitsVerifyMaximumMessageSize(t *testing.T) {
	limits := queue.NewLimits(queue.StrictMode)

	err := limits.VerifyMaximumMessageSize(262144)
	assert.NoError(t, err)

	err = limits.VerifyMaximumMessageSize(100)
	assert.Error(t, err)
}
