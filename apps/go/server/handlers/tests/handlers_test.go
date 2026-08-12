package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/apps/go/server/handlers"
	"github.com/tguidoux/opensqs/apps/go/server/protocol"
	"github.com/tguidoux/opensqs/pkgs/v1/logger"
	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// mockRequest implements the handlers.Request interface for testing.
type mockRequest struct {
	action                string
	queueURL              string
	queueName             string
	messageBody           string
	delaySeconds          int
	visibilityTimeout     int
	maxNumberOfMessages   int
	waitTimeSeconds       int
	receiptHandle         string
	prefix                string
	attributeNames        []string
	messageAttributes     map[string]types.MessageAttribute
	messageAttributeNames []string
	attributes            map[string]string
	batchEntries          []handlers.BatchEntry
	tags                  map[string]string
	tagKeys               []string
	dedupID               string
	groupID               string
	systemAttributes      map[string]types.MessageSystemAttribute
	sourceArn             string
	destinationArn        string
	taskHandle            string
	maxMoveRate           int
}

func (m *mockRequest) GetAction() string           { return m.action }
func (m *mockRequest) GetQueueURL() string         { return m.queueURL }
func (m *mockRequest) GetQueueName() string        { return m.queueName }
func (m *mockRequest) GetMessageBody() string      { return m.messageBody }
func (m *mockRequest) GetDelaySeconds() int        { return m.delaySeconds }
func (m *mockRequest) GetVisibilityTimeout() int   { return m.visibilityTimeout }
func (m *mockRequest) GetMaxNumberOfMessages() int { return m.maxNumberOfMessages }
func (m *mockRequest) GetWaitTimeSeconds() int     { return m.waitTimeSeconds }
func (m *mockRequest) GetReceiptHandle() string    { return m.receiptHandle }
func (m *mockRequest) GetPrefix() string           { return m.prefix }
func (m *mockRequest) GetAttributeNames() []string { return m.attributeNames }
func (m *mockRequest) GetMessageAttributes() map[string]types.MessageAttribute {
	return m.messageAttributes
}
func (m *mockRequest) GetMessageAttributeNames() []string     { return m.messageAttributeNames }
func (m *mockRequest) GetAttributes() map[string]string       { return m.attributes }
func (m *mockRequest) GetBatchEntries() []handlers.BatchEntry { return m.batchEntries }
func (m *mockRequest) GetTags() map[string]string             { return m.tags }
func (m *mockRequest) GetTagKeys() []string                   { return m.tagKeys }
func (m *mockRequest) GetMessageDeduplicationID() string      { return m.dedupID }
func (m *mockRequest) GetMessageGroupID() string              { return m.groupID }
func (m *mockRequest) GetMessageSystemAttributes() map[string]types.MessageSystemAttribute {
	return m.systemAttributes
}
func (m *mockRequest) GetSourceArn() string                 { return m.sourceArn }
func (m *mockRequest) GetDestinationArn() string            { return m.destinationArn }
func (m *mockRequest) GetTaskHandle() string                { return m.taskHandle }
func (m *mockRequest) GetMaxNumberOfMessagesPerSecond() int { return m.maxMoveRate }

func newTestHandler() *handlers.Handler {
	factory := func(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) (store.Store, error) {
		return memory.NewMemoryStore(queueName, visibilityTimeout, serverSecret, cfg), nil
	}
	manager := queue.NewQueueManager("localhost:9324", "123456789012", "us-east-1", []byte("test-secret"), factory, logger.New("test", logger.UncontextualLoggerType))
	limits := queue.NewLimits(queue.StrictMode)
	return handlers.NewHandler(manager, limits, false, nil, logger.New("test", logger.UncontextualLoggerType))
}

func TestHandleRequest_CreateQueue(t *testing.T) {
	h := newTestHandler()
	req := &mockRequest{
		action:    "CreateQueue",
		queueName: "test-queue",
	}

	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "CreateQueue", resp.Action)
	assert.Equal(t, "http://localhost:9324/123456789012/test-queue", resp.QueueURL)
}

func TestHandleRequest_CreateQueue_EmptyName(t *testing.T) {
	h := newTestHandler()
	req := &mockRequest{
		action:    "CreateQueue",
		queueName: "",
	}

	_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	assert.Error(t, err)
}

func TestHandleRequest_CreateQueue_Duplicate(t *testing.T) {
	h := newTestHandler()
	req := &mockRequest{
		action:    "CreateQueue",
		queueName: "dup-queue",
	}

	_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)

	// Creating the same queue again should succeed (idempotent)
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9324/123456789012/dup-queue", resp.QueueURL)
}

func TestHandleRequest_GetQueueURL(t *testing.T) {
	h := newTestHandler()

	// Create a queue first
	createReq := &mockRequest{action: "CreateQueue", queueName: "url-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Get the URL
	req := &mockRequest{action: "GetQueueUrl", queueName: "url-test"}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9324/123456789012/url-test", resp.QueueURL)
}

func TestHandleRequest_GetQueueURL_NotFound(t *testing.T) {
	h := newTestHandler()
	req := &mockRequest{action: "GetQueueUrl", queueName: "nonexistent"}

	_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	assert.Error(t, err)
}

func TestHandleRequest_ListQueues(t *testing.T) {
	h := newTestHandler()

	// Create some queues
	for _, name := range []string{"q1", "q2", "q3"} {
		req := &mockRequest{action: "CreateQueue", queueName: name}
		_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
		require.NoError(t, err)
	}

	// List all
	req := &mockRequest{action: "ListQueues"}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Len(t, resp.QueueURLs, 3)
}

func TestHandleRequest_ListQueues_Prefix(t *testing.T) {
	h := newTestHandler()

	for _, name := range []string{"prefix-a", "prefix-b", "other"} {
		req := &mockRequest{action: "CreateQueue", queueName: name}
		_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
		require.NoError(t, err)
	}

	req := &mockRequest{action: "ListQueues", prefix: "prefix"}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Len(t, resp.QueueURLs, 2)
}

func TestHandleRequest_SendMessage(t *testing.T) {
	h := newTestHandler()

	// Create queue first
	createReq := &mockRequest{action: "CreateQueue", queueName: "send-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Send message
	req := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/send-test",
		messageBody: "hello world",
	}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "SendMessage", resp.Action)
	assert.NotNil(t, resp.Message)
	assert.NotEmpty(t, resp.Message.MessageID)
	assert.NotEmpty(t, resp.Message.MD5OfBody)
}

func TestHandleRequest_SendMessage_EmptyBody(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "empty-body-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	req := &mockRequest{
		action:   "SendMessage",
		queueURL: "http://localhost:9324/123456789012/empty-body-test",
	}
	_, err = h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	assert.Error(t, err)
}

func TestHandleRequest_SendMessage_QueueNotFound(t *testing.T) {
	h := newTestHandler()
	req := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/nonexistent",
		messageBody: "hello",
	}
	_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	assert.Error(t, err)
}

func TestHandleRequest_ReceiveMessage(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "recv-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Send a message first
	sendReq := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/recv-test",
		messageBody: "hello",
	}
	_, err = h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Receive
	req := &mockRequest{
		action:              "ReceiveMessage",
		queueURL:            "http://localhost:9324/123456789012/recv-test",
		maxNumberOfMessages: 1,
		visibilityTimeout:   30,
		waitTimeSeconds:     0,
	}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Len(t, resp.Messages, 1)
	assert.Equal(t, "hello", resp.Messages[0].Body)
	assert.NotEmpty(t, resp.Messages[0].ReceiptHandle)
}

func TestHandleRequest_ReceiveMessage_EmptyQueue(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "empty-queue"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	req := &mockRequest{
		action:              "ReceiveMessage",
		queueURL:            "http://localhost:9324/123456789012/empty-queue",
		maxNumberOfMessages: 1,
		visibilityTimeout:   30,
		waitTimeSeconds:     0,
	}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Empty(t, resp.Messages)
}

func TestHandleRequest_DeleteMessage(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "del-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Send
	sendReq := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/del-test",
		messageBody: "hello",
	}
	_, err = h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Receive to get receipt handle
	recvReq := &mockRequest{
		action:              "ReceiveMessage",
		queueURL:            "http://localhost:9324/123456789012/del-test",
		maxNumberOfMessages: 1,
		visibilityTimeout:   30,
		waitTimeSeconds:     0,
	}
	recvResp, err := h.HandleRequest(context.Background(), recvReq, handlers.QueryProtocol)
	require.NoError(t, err)
	require.Len(t, recvResp.Messages, 1)

	// Delete
	delReq := &mockRequest{
		action:        "DeleteMessage",
		queueURL:      "http://localhost:9324/123456789012/del-test",
		receiptHandle: recvResp.Messages[0].ReceiptHandle,
	}
	resp, err := h.HandleRequest(context.Background(), delReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "DeleteMessage", resp.Action)
}

func TestHandleRequest_DeleteMessage_EmptyHandle(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "del-empty"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	req := &mockRequest{
		action:   "DeleteMessage",
		queueURL: "http://localhost:9324/123456789012/del-empty",
	}
	_, err = h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	assert.Error(t, err)
}

func TestHandleRequest_GetQueueAttributes(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "attrs-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	req := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/attrs-test",
		attributeNames: []string{"VisibilityTimeout", "QueueArn"},
	}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Contains(t, resp.Attributes, "VisibilityTimeout")
	assert.Contains(t, resp.Attributes, "QueueArn")
	assert.Equal(t, "30", resp.Attributes["VisibilityTimeout"])
}

func TestHandleRequest_GetQueueAttributes_All(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "all-attrs-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	req := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/all-attrs-test",
		attributeNames: []string{"All"},
	}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Contains(t, resp.Attributes, "VisibilityTimeout")
	assert.Contains(t, resp.Attributes, "QueueArn")
	assert.Contains(t, resp.Attributes, "DelaySeconds")
	assert.Contains(t, resp.Attributes, "MaximumMessageSize")
}

func TestHandleRequest_PurgeQueue(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "purge-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Send some messages
	for i := 0; i < 3; i++ {
		sendReq := &mockRequest{
			action:      "SendMessage",
			queueURL:    "http://localhost:9324/123456789012/purge-test",
			messageBody: "msg",
		}
		_, err := h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
		require.NoError(t, err)
	}

	// Purge
	req := &mockRequest{
		action:   "PurgeQueue",
		queueURL: "http://localhost:9324/123456789012/purge-test",
	}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "PurgeQueue", resp.Action)

	// Verify queue is empty
	recvReq := &mockRequest{
		action:              "ReceiveMessage",
		queueURL:            "http://localhost:9324/123456789012/purge-test",
		maxNumberOfMessages: 10,
		visibilityTimeout:   30,
		waitTimeSeconds:     0,
	}
	recvResp, err := h.HandleRequest(context.Background(), recvReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Empty(t, recvResp.Messages)
}

func TestHandleRequest_DeleteQueue(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "delete-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Delete
	req := &mockRequest{
		action:   "DeleteQueue",
		queueURL: "http://localhost:9324/123456789012/delete-test",
	}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "DeleteQueue", resp.Action)

	// Verify queue is gone
	getReq := &mockRequest{action: "GetQueueUrl", queueName: "delete-test"}
	_, err = h.HandleRequest(context.Background(), getReq, handlers.QueryProtocol)
	assert.Error(t, err)
}

func TestHandleRequest_InvalidAction(t *testing.T) {
	h := newTestHandler()
	req := &mockRequest{action: "InvalidAction"}

	_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	assert.Error(t, err)
}

func TestHandleRequest_ChangeMessageVisibility(t *testing.T) {
	h := newTestHandler()

	createReq := &mockRequest{action: "CreateQueue", queueName: "vis-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Send
	sendReq := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/vis-test",
		messageBody: "hello",
	}
	_, err = h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Receive
	recvReq := &mockRequest{
		action:              "ReceiveMessage",
		queueURL:            "http://localhost:9324/123456789012/vis-test",
		maxNumberOfMessages: 1,
		visibilityTimeout:   30,
		waitTimeSeconds:     0,
	}
	recvResp, err := h.HandleRequest(context.Background(), recvReq, handlers.QueryProtocol)
	require.NoError(t, err)
	require.Len(t, recvResp.Messages, 1)

	// Change visibility
	visReq := &mockRequest{
		action:            "ChangeMessageVisibility",
		queueURL:          "http://localhost:9324/123456789012/vis-test",
		receiptHandle:     recvResp.Messages[0].ReceiptHandle,
		visibilityTimeout: 60,
	}
	resp, err := h.HandleRequest(context.Background(), visReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "ChangeMessageVisibility", resp.Action)
}

// ---------------------------------------------------------------------------
// Response Marshalling Tests
// ---------------------------------------------------------------------------

func TestMarshalResponse_XML_CreateQueue(t *testing.T) {
	h := newTestHandler()
	createReq := &mockRequest{action: "CreateQueue", queueName: "marshal-test"}
	resp, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	data, err := handlers.MarshalResponse(resp, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<CreateQueueResponse>")
	assert.Contains(t, string(data), "marshal-test")
}

func TestMarshalResponse_JSON_CreateQueue(t *testing.T) {
	h := newTestHandler()
	createReq := &mockRequest{action: "CreateQueue", queueName: "json-marshal-test"}
	resp, err := h.HandleRequest(context.Background(), createReq, handlers.JSONProtocol)
	require.NoError(t, err)

	data, err := handlers.MarshalResponse(resp, handlers.JSONProtocol)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"QueueUrl"`)
	assert.Contains(t, string(data), "json-marshal-test")
}

func TestMarshalResponse_XML_ListQueues(t *testing.T) {
	h := newTestHandler()
	for _, name := range []string{"m1", "m2"} {
		req := &mockRequest{action: "CreateQueue", queueName: name}
		_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
		require.NoError(t, err)
	}

	req := &mockRequest{action: "ListQueues"}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)

	data, err := handlers.MarshalResponse(resp, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<ListQueuesResponse>")
	assert.Contains(t, string(data), "m1")
	assert.Contains(t, string(data), "m2")
}

func TestMarshalResponse_XML_SendMessage(t *testing.T) {
	h := newTestHandler()
	createReq := &mockRequest{action: "CreateQueue", queueName: "xml-send-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	sendReq := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/xml-send-test",
		messageBody: "hello",
	}
	resp, err := h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)

	data, err := handlers.MarshalResponse(resp, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<SendMessageResponse>")
	assert.Contains(t, string(data), "<MessageId>")
	assert.Contains(t, string(data), "<MD5OfMessageBody>")
}

func TestMarshalResponse_XML_ReceiveMessage(t *testing.T) {
	h := newTestHandler()
	createReq := &mockRequest{action: "CreateQueue", queueName: "xml-recv-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	sendReq := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/xml-recv-test",
		messageBody: "hello world",
	}
	_, err = h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)

	recvReq := &mockRequest{
		action:              "ReceiveMessage",
		queueURL:            "http://localhost:9324/123456789012/xml-recv-test",
		maxNumberOfMessages: 1,
		visibilityTimeout:   30,
		waitTimeSeconds:     0,
	}
	resp, err := h.HandleRequest(context.Background(), recvReq, handlers.QueryProtocol)
	require.NoError(t, err)

	data, err := handlers.MarshalResponse(resp, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<ReceiveMessageResponse>")
	assert.Contains(t, string(data), "<Body>hello world</Body>")
	assert.Contains(t, string(data), "<ReceiptHandle>")
}

func TestMarshalResponse_JSON_SendMessage(t *testing.T) {
	h := newTestHandler()
	createReq := &mockRequest{action: "CreateQueue", queueName: "json-send-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	sendReq := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/json-send-test",
		messageBody: "hello",
	}
	resp, err := h.HandleRequest(context.Background(), sendReq, handlers.JSONProtocol)
	require.NoError(t, err)

	data, err := handlers.MarshalResponse(resp, handlers.JSONProtocol)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"MessageId"`)
	assert.Contains(t, string(data), `"MD5OfMessageBody"`)
}

func TestMarshalResponse_XML_GetQueueAttributes(t *testing.T) {
	h := newTestHandler()
	createReq := &mockRequest{action: "CreateQueue", queueName: "xml-attrs-test"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	require.NoError(t, err)

	req := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/xml-attrs-test",
		attributeNames: []string{"All"},
	}
	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)

	data, err := handlers.MarshalResponse(resp, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Contains(t, string(data), "<GetQueueAttributesResponse>")
	assert.Contains(t, string(data), "VisibilityTimeout")
}

func TestMarshalResponse_ErrorResponse(t *testing.T) {
	h := newTestHandler()
	req := &mockRequest{action: "GetQueueUrl", queueName: "nonexistent"}

	_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	assert.Error(t, err)

	// Verify the error implements the SQS error interface
	sqsErr, ok := err.(*types.ConcreteSQSError)
	if ok {
		assert.NotEmpty(t, sqsErr.Code(), "error should have a non-empty code")
		assert.NotEmpty(t, sqsErr.Message(), "error should have a non-empty message")
		assert.Greater(t, sqsErr.HTTPStatusCode(), 0, "error should have a valid HTTP status code")
	}
}

// ---------------------------------------------------------------------------
// Protocol Adapter Tests
// ---------------------------------------------------------------------------

func TestQueryRequestAdapter(t *testing.T) {
	h := newTestHandler()

	// Create a queue using the query protocol adapter
	queryReq, err := protocol.ParseQueryRequest("Action=CreateQueue&QueueName=adapter-test")
	require.NoError(t, err)

	adapter := &handlers.QueryRequestAdapter{QueryRequest: queryReq}
	resp, err := h.HandleRequest(context.Background(), adapter, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9324/123456789012/adapter-test", resp.QueueURL)
}

func TestJSONRequestAdapter(t *testing.T) {
	h := newTestHandler()

	jsonReq, err := protocol.ParseJSONRequest("AmazonSQS.CreateQueue", []byte(`{"QueueName": "json-adapter-test"}`))
	require.NoError(t, err)

	adapter := &handlers.JSONRequestAdapter{JSONRequest: jsonReq}
	resp, err := h.HandleRequest(context.Background(), adapter, handlers.JSONProtocol)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9324/123456789012/json-adapter-test", resp.QueueURL)
}
