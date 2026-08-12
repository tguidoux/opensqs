package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func TestSQSVersion_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, types.SQSVersion, "SQSVersion must not be empty")
	assert.Contains(t, types.SQSVersion, "-", "SQSVersion should be a date format")
}

func TestActionConstants_AreDistinct(t *testing.T) {
	actions := []string{
		types.ActionCreateQueue, types.ActionSendMessage, types.ActionReceiveMessage,
		types.ActionDeleteMessage, types.ActionPurgeQueue,
	}
	seen := make(map[string]bool)
	for _, a := range actions {
		assert.False(t, seen[a], "duplicate action constant: %s", a)
		assert.NotEmpty(t, a, "action constant must not be empty")
		seen[a] = true
	}
}

func TestAttributeConstants_AreDistinct(t *testing.T) {
	attrs := []string{
		types.AttributeVisibilityTimeout, types.AttributeDelaySeconds,
		types.AttributeMaximumMessageSize, types.AttributeMessageRetentionPeriod,
		types.AttributeQueueArn,
	}
	seen := make(map[string]bool)
	for _, a := range attrs {
		assert.False(t, seen[a], "duplicate attribute constant: %s", a)
		seen[a] = true
	}
}

func TestContentTypeConstants_MatchExpectedFormats(t *testing.T) {
	assert.Contains(t, types.QueryProtocolContentType, "form-urlencoded")
	assert.Contains(t, types.JSONProtocolContentType, "json")
	assert.Contains(t, types.XMLContentType, "xml")
}

func TestMessageAttributeTypes_AreDistinct(t *testing.T) {
	assert.NotEqual(t, types.MessageAttributeTypeString, types.MessageAttributeTypeNumber)
	assert.NotEqual(t, types.MessageAttributeTypeString, types.MessageAttributeTypeBinary)
	assert.NotEqual(t, types.MessageAttributeTypeNumber, types.MessageAttributeTypeBinary)
}

func TestDefaultLimits_AreWithinMaxLimits(t *testing.T) {
	assert.LessOrEqual(t, types.DefaultVisibilityTimeout, types.MaxVisibilityTimeout)
	assert.LessOrEqual(t, types.DefaultMessageRetentionPeriod, types.MaxMessageRetentionPeriod)
	assert.LessOrEqual(t, types.DefaultMaximumMessageSize, types.MaxMaximumMessageSize)
	assert.LessOrEqual(t, types.DefaultDelaySeconds, types.MaxDelaySeconds)
	assert.LessOrEqual(t, types.DefaultReceiveMessageWaitTime, types.MaxReceiveMessageWaitTime)
}

func TestMaxLimits_ArePositive(t *testing.T) {
	assert.Positive(t, types.MaxVisibilityTimeout)
	assert.Positive(t, types.MaxMessageRetentionPeriod)
	assert.Positive(t, types.MaxMaximumMessageSize)
	assert.Positive(t, types.MaxDelaySeconds)
	assert.Positive(t, types.MaxReceiveMessageWaitTime)
	assert.Positive(t, types.MaxNumberOfMessages)
}

func TestMessageStruct(t *testing.T) {
	msg := types.Message{
		MessageID: "msg-123",
		Body:      "hello world",
	}

	assert.Equal(t, "msg-123", msg.MessageID)
	assert.Equal(t, "hello world", msg.Body)
}

func TestMessageAttributeStruct(t *testing.T) {
	attr := types.MessageAttribute{
		DataType:    types.MessageAttributeTypeString,
		StringValue: "my-value",
	}

	assert.Equal(t, "String", attr.DataType)
	assert.Equal(t, "my-value", attr.StringValue)
}

func TestMessageAttributeBinary(t *testing.T) {
	attr := types.MessageAttribute{
		DataType:    types.MessageAttributeTypeBinary,
		BinaryValue: []byte{0x01, 0x02, 0x03},
	}

	assert.Equal(t, "Binary", attr.DataType)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, attr.BinaryValue)
}

func TestQueueAttributesMap(t *testing.T) {
	attrs := types.QueueAttributes{
		types.AttributeVisibilityTimeout: "30",
		types.AttributeDelaySeconds:      "0",
	}

	assert.Equal(t, "30", attrs[types.AttributeVisibilityTimeout])
	assert.Equal(t, "0", attrs[types.AttributeDelaySeconds])
}

func TestReceiptHandleInfoStruct(t *testing.T) {
	info := types.ReceiptHandleInfo{
		QueueName:   "my-queue",
		MessageID:   "msg-456",
		RandomNonce: "abc123",
	}

	assert.Equal(t, "my-queue", info.QueueName)
	assert.Equal(t, "msg-456", info.MessageID)
	assert.Equal(t, "abc123", info.RandomNonce)
}
