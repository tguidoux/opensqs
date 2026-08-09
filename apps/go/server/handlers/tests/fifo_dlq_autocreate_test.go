package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/apps/go/server/handlers"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// newAutoCreateTestHandler creates a handler with autoCreate enabled.
func newAutoCreateTestHandler() *handlers.Handler {
	factory := func(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) (store.Store, error) {
		return memory.NewMemoryStore(queueName, visibilityTimeout, serverSecret, cfg), nil
	}
	manager := queue.NewQueueManager("localhost:9324", "123456789012", "us-east-1", []byte("test-secret"), factory)
	limits := queue.NewLimits(queue.StrictMode)
	return handlers.NewHandler(manager, limits, true, nil)
}

// ---------------------------------------------------------------------------
// Auto-Create Tests
// ---------------------------------------------------------------------------

func TestAutoCreate_SendToNonExistentQueue(t *testing.T) {
	h := newAutoCreateTestHandler()

	// Send to a queue that doesn't exist — should auto-create it
	sendReq := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/auto-created",
		messageBody: "hello",
	}
	resp, err := h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "SendMessage", resp.Action)
	require.NotNil(t, resp.Message)
	assert.NotEmpty(t, resp.Message.MessageID)

	// Verify the queue was created by listing queues
	listReq := &mockRequest{action: "ListQueues"}
	listResp, err := h.HandleRequest(context.Background(), listReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Contains(t, listResp.QueueURLs, "http://localhost:9324/123456789012/auto-created")
}

func TestAutoCreate_ReceiveFromNonExistentQueue(t *testing.T) {
	h := newAutoCreateTestHandler()

	// Receive from a queue that doesn't exist — should auto-create it
	recvReq := &mockRequest{
		action:              "ReceiveMessage",
		queueURL:            "http://localhost:9324/123456789012/auto-recv",
		maxNumberOfMessages: 1,
		visibilityTimeout:   30,
		waitTimeSeconds:     0,
	}
	resp, err := h.HandleRequest(context.Background(), recvReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Empty(t, resp.Messages)

	// Queue should now exist
	listReq := &mockRequest{action: "ListQueues"}
	listResp, err := h.HandleRequest(context.Background(), listReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Contains(t, listResp.QueueURLs, "http://localhost:9324/123456789012/auto-recv")
}

func TestAutoCreate_Disabled_SendToNonExistentQueue(t *testing.T) {
	h := newTestHandler() // autoCreate=false

	sendReq := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/no-auto-create",
		messageBody: "hello",
	}
	_, err := h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FIFO Queue Tests
// ---------------------------------------------------------------------------

func TestFIFO_CreateQueue(t *testing.T) {
	h := newTestHandler()

	req := &mockRequest{
		action:    "CreateQueue",
		queueName: "fifo-test.fifo",
		attributes: map[string]string{
			"FifoQueue":                 "true",
			"ContentBasedDeduplication": "true",
		},
	}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9324/123456789012/fifo-test.fifo", resp.QueueURL)
}

func TestFIFO_SendMessageWithGroupAndDedup(t *testing.T) {
	h := newTestHandler()

	// Create FIFO queue
	createReq := &mockRequest{
		action:    "CreateQueue",
		queueName: "fifo-send.fifo",
		attributes: map[string]string{
			"FifoQueue": "true",
		},
	}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Send message with group ID and dedup ID
	sendReq := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/fifo-send.fifo",
		messageBody: "hello",
		dedupID:     "dedup-1",
		groupID:     "group-1",
	}
	resp, err := h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "SendMessage", resp.Action)
	require.NotNil(t, resp.Message)
	assert.NotEmpty(t, resp.Message.MessageID)
	assert.NotEmpty(t, resp.Message.SequenceNumber)
}

func TestFIFO_SendMessageBatch(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{
		action:    "CreateQueue",
		queueName: "fifo-batch.fifo",
		attributes: map[string]string{
			"FifoQueue": "true",
		},
	}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	entries := []handlers.BatchEntry{
		{ID: "1", MessageBody: "msg1", MessageDeduplicationID: "d1", MessageGroupID: "g1"},
		{ID: "2", MessageBody: "msg2", MessageDeduplicationID: "d2", MessageGroupID: "g1"},
	}

	sendReq := &mockRequest{
		action:       "SendMessageBatch",
		queueURL:     "http://localhost:9324/123456789012/fifo-batch.fifo",
		batchEntries: entries,
	}
	resp, err := h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Len(t, resp.BatchResults, 2)
	for _, r := range resp.BatchResults {
		assert.NotEmpty(t, r.MessageID)
		assert.NotEmpty(t, r.SequenceNumber)
	}
}

// ---------------------------------------------------------------------------
// System Attributes Tests
// ---------------------------------------------------------------------------

func TestSystemAttributes_SendMessage(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "sys-attrs-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	sysAttrs := map[string]types.MessageSystemAttribute{
		"AWSTraceHeader": {
			DataType:    "String",
			StringValue: "Root=1-5759e988-bd862e3fe1be46a994272793;Sampled=1",
		},
	}

	sendReq := &mockRequest{
		action:           "SendMessage",
		queueURL:         "http://localhost:9324/123456789012/sys-attrs-test",
		messageBody:      "hello",
		systemAttributes: sysAttrs,
	}
	resp, err := h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)
	require.NotNil(t, resp.Message)
	assert.NotEmpty(t, resp.Message.MD5OfMessageSystemAttributes)
}

func TestSystemAttributes_SendMessageBatch(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "sys-attrs-batch"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	sysAttrs := map[string]types.MessageSystemAttribute{
		"AWSTraceHeader": {
			DataType:    "String",
			StringValue: "Root=1-5759e988-bd862e3fe1be46a994272793",
		},
	}

	entries := []handlers.BatchEntry{
		{ID: "1", MessageBody: "msg1", MessageSystemAttributes: sysAttrs},
		{ID: "2", MessageBody: "msg2"},
	}

	sendReq := &mockRequest{
		action:       "SendMessageBatch",
		queueURL:     "http://localhost:9324/123456789012/sys-attrs-batch",
		batchEntries: entries,
	}
	resp, err := h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)
	require.Len(t, resp.BatchResults, 2)

	// First entry had system attributes
	assert.NotEmpty(t, resp.BatchResults[0].MD5OfMessageSystemAttributes)
	// Second entry had no system attributes
	assert.Empty(t, resp.BatchResults[1].MD5OfMessageSystemAttributes)
}

func TestSystemAttributes_ReceiveMessage(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "sys-attrs-recv"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	sysAttrs := map[string]types.MessageSystemAttribute{
		"AWSTraceHeader": {
			DataType:    "String",
			StringValue: "Root=1-5759e988-bd862e3fe1be46a994272793",
		},
	}

	sendReq := &mockRequest{
		action:           "SendMessage",
		queueURL:         "http://localhost:9324/123456789012/sys-attrs-recv",
		messageBody:      "hello",
		systemAttributes: sysAttrs,
	}
	_, err = h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)

	recvReq := &mockRequest{
		action:              "ReceiveMessage",
		queueURL:            "http://localhost:9324/123456789012/sys-attrs-recv",
		maxNumberOfMessages: 1,
		visibilityTimeout:   30,
		waitTimeSeconds:     0,
	}
	resp, err := h.HandleRequest(context.Background(), recvReq, handlers.QueryProtocol)
	require.NoError(t, err)
	require.Len(t, resp.Messages, 1)
	assert.NotEmpty(t, resp.Messages[0].SystemAttributes)
	assert.Contains(t, resp.Messages[0].SystemAttributes, "AWSTraceHeader")
}

// ---------------------------------------------------------------------------
// DLQ Handler Tests
// ---------------------------------------------------------------------------

func TestDLQ_CreateQueueWithRedrivePolicy(t *testing.T) {
	h := newTestHandler()

	// Create the DLQ first
	dlqReq := &mockRequest{action: "CreateQueue", queueName: "my-dlq"}
	_, err := h.HandleRequest(context.Background(), dlqReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Get the DLQ's ARN
	dlqAttrsReq := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/my-dlq",
		attributeNames: []string{"QueueArn"},
	}
	dlqAttrsResp, err := h.HandleRequest(context.Background(), dlqAttrsReq, handlers.QueryProtocol)
	require.NoError(t, err)
	dlqArn := dlqAttrsResp.Attributes["QueueArn"]
	require.NotEmpty(t, dlqArn)

	// Create main queue with redrive policy pointing to DLQ
	redrivePolicy := `{"deadLetterTargetArn":"` + dlqArn + `","maxReceiveCount":"3"}`
	mainReq := &mockRequest{
		action:    "CreateQueue",
		queueName: "main-queue",
		attributes: map[string]string{
			"RedrivePolicy": redrivePolicy,
		},
	}
	resp, err := h.HandleRequest(context.Background(), mainReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9324/123456789012/main-queue", resp.QueueURL)
}

func TestDLQ_GetQueueAttributes_RedrivePolicy(t *testing.T) {
	h := newTestHandler()

	// Create DLQ
	dlqReq := &mockRequest{action: "CreateQueue", queueName: "attrs-dlq"}
	_, err := h.HandleRequest(context.Background(), dlqReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Get DLQ ARN
	dlqAttrsReq := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/attrs-dlq",
		attributeNames: []string{"QueueArn"},
	}
	dlqAttrsResp, err := h.HandleRequest(context.Background(), dlqAttrsReq, handlers.QueryProtocol)
	require.NoError(t, err)
	dlqArn := dlqAttrsResp.Attributes["QueueArn"]
	require.NotEmpty(t, dlqArn)

	// Create main queue with redrive policy
	redrivePolicy := `{"deadLetterTargetArn":"` + dlqArn + `","maxReceiveCount":"2"}`
	mainReq := &mockRequest{
		action:    "CreateQueue",
		queueName: "attrs-main",
		attributes: map[string]string{
			"RedrivePolicy": redrivePolicy,
		},
	}
	_, err = h.HandleRequest(context.Background(), mainReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Get attributes — should include RedrivePolicy
	getReq := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/attrs-main",
		attributeNames: []string{"RedrivePolicy"},
	}
	resp, err := h.HandleRequest(context.Background(), getReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Contains(t, resp.Attributes, "RedrivePolicy")
}

// ---------------------------------------------------------------------------
// Marshal Tests for New Features
// ---------------------------------------------------------------------------

func TestMarshalResponse_XML_SendMessage_WithSystemAttrs(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "xml-sys-attrs"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	sysAttrs := map[string]types.MessageSystemAttribute{
		"AWSTraceHeader": {
			DataType:    "String",
			StringValue: "Root=1-test",
		},
	}

	sendReq := &mockRequest{
		action:           "SendMessage",
		queueURL:         "http://localhost:9324/123456789012/xml-sys-attrs",
		messageBody:      "hello",
		systemAttributes: sysAttrs,
	}
	resp, err := h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)

	data, err := handlers.MarshalResponse(resp, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<SendMessageResponse>")
	assert.Contains(t, string(data), "<MD5OfMessageSystemAttributes>")
}

func TestMarshalResponse_JSON_SendMessage_WithSystemAttrs(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "json-sys-attrs"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	sysAttrs := map[string]types.MessageSystemAttribute{
		"AWSTraceHeader": {
			DataType:    "String",
			StringValue: "Root=1-test",
		},
	}

	sendReq := &mockRequest{
		action:           "SendMessage",
		queueURL:         "http://localhost:9324/123456789012/json-sys-attrs",
		messageBody:      "hello",
		systemAttributes: sysAttrs,
	}
	resp, err := h.HandleRequest(context.Background(), sendReq, handlers.JSONProtocol)
	require.NoError(t, err)

	data, err := handlers.MarshalResponse(resp, handlers.JSONProtocol)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"MD5OfMessageSystemAttributes"`)
}

func TestMarshalResponse_XML_SendMessageBatch_FIFO(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{
		action:    "CreateQueue",
		queueName: "xml-fifo-batch.fifo",
		attributes: map[string]string{
			"FifoQueue": "true",
		},
	}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	entries := []handlers.BatchEntry{
		{ID: "1", MessageBody: "msg1", MessageDeduplicationID: "d1", MessageGroupID: "g1"},
		{ID: "2", MessageBody: "msg2", MessageDeduplicationID: "d2", MessageGroupID: "g1"},
	}

	sendReq := &mockRequest{
		action:       "SendMessageBatch",
		queueURL:     "http://localhost:9324/123456789012/xml-fifo-batch.fifo",
		batchEntries: entries,
	}
	resp, err := h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)

	data, err := handlers.MarshalResponse(resp, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<SendMessageBatchResponse>")
	assert.Contains(t, string(data), "<SequenceNumber>")
}
