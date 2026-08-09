package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func TestSQSVersion(t *testing.T) {
	assert.Equal(t, "2012-11-05", types.SQSVersion)
}

func TestActionConstants(t *testing.T) {
	assert.Equal(t, "CreateQueue", types.ActionCreateQueue)
	assert.Equal(t, "SendMessage", types.ActionSendMessage)
	assert.Equal(t, "ReceiveMessage", types.ActionReceiveMessage)
	assert.Equal(t, "DeleteMessage", types.ActionDeleteMessage)
	assert.Equal(t, "PurgeQueue", types.ActionPurgeQueue)
}

func TestAttributeConstants(t *testing.T) {
	assert.Equal(t, "VisibilityTimeout", types.AttributeVisibilityTimeout)
	assert.Equal(t, "DelaySeconds", types.AttributeDelaySeconds)
	assert.Equal(t, "MaximumMessageSize", types.AttributeMaximumMessageSize)
	assert.Equal(t, "MessageRetentionPeriod", types.AttributeMessageRetentionPeriod)
	assert.Equal(t, "QueueArn", types.AttributeQueueArn)
}

func TestContentTypeConstants(t *testing.T) {
	assert.Equal(t, "application/x-www-form-urlencoded", types.QueryProtocolContentType)
	assert.Equal(t, "application/x-amz-json-1.0", types.JSONProtocolContentType)
	assert.Equal(t, "text/xml", types.XMLContentType)
}

func TestMessageAttributeTypes(t *testing.T) {
	assert.Equal(t, "String", types.MessageAttributeTypeString)
	assert.Equal(t, "Number", types.MessageAttributeTypeNumber)
	assert.Equal(t, "Binary", types.MessageAttributeTypeBinary)
}

func TestDefaultLimits(t *testing.T) {
	assert.Equal(t, 30, types.DefaultVisibilityTimeout)
	assert.Equal(t, 345600, types.DefaultMessageRetentionPeriod)
	assert.Equal(t, 262144, types.DefaultMaximumMessageSize)
	assert.Equal(t, 0, types.DefaultDelaySeconds)
	assert.Equal(t, 0, types.DefaultReceiveMessageWaitTime)
}

func TestMaxLimits(t *testing.T) {
	assert.Equal(t, 43200, types.MaxVisibilityTimeout)
	assert.Equal(t, 1209600, types.MaxMessageRetentionPeriod)
	assert.Equal(t, 262144, types.MaxMaximumMessageSize)
	assert.Equal(t, 900, types.MaxDelaySeconds)
	assert.Equal(t, 20, types.MaxReceiveMessageWaitTime)
	assert.Equal(t, 10, types.MaxNumberOfMessages)
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
