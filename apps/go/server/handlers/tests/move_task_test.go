package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tguidoux/opensqs/apps/go/server/handlers"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

// ---------------------------------------------------------------------------
// StartMessageMoveTask Tests
// ---------------------------------------------------------------------------

func TestStartMessageMoveTask_MissingSourceArn(t *testing.T) {
	h := newTestHandler()

	req := &mockRequest{
		action: types.ActionStartMessageMoveTask,
	}

	_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SourceArn")
}

func TestStartMessageMoveTask_SourceQueueNotFound(t *testing.T) {
	h := newTestHandler()

	req := &mockRequest{
		action:    types.ActionStartMessageMoveTask,
		sourceArn: "arn:aws:sqs:us-east-1:123456789012:nonexistent",
	}

	_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	assert.Error(t, err)
}

func TestStartMessageMoveTask_WithExplicitDestination(t *testing.T) {
	h := newTestHandler()

	// Create source queue
	createSrc := &mockRequest{action: "CreateQueue", queueName: "move-src"}
	_, err := h.HandleRequest(context.Background(), createSrc, handlers.QueryProtocol)
	require.NoError(t, err)

	// Get source ARN
	srcAttrsReq := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/move-src",
		attributeNames: []string{"QueueArn"},
	}
	srcAttrsResp, err := h.HandleRequest(context.Background(), srcAttrsReq, handlers.QueryProtocol)
	require.NoError(t, err)
	srcArn := srcAttrsResp.Attributes["QueueArn"]
	require.NotEmpty(t, srcArn)

	// Create destination queue
	createDst := &mockRequest{action: "CreateQueue", queueName: "move-dst"}
	_, err = h.HandleRequest(context.Background(), createDst, handlers.QueryProtocol)
	require.NoError(t, err)

	// Get destination ARN
	dstAttrsReq := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/move-dst",
		attributeNames: []string{"QueueArn"},
	}
	dstAttrsResp, err := h.HandleRequest(context.Background(), dstAttrsReq, handlers.QueryProtocol)
	require.NoError(t, err)
	dstArn := dstAttrsResp.Attributes["QueueArn"]
	require.NotEmpty(t, dstArn)

	// Send a message to source
	sendReq := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/move-src",
		messageBody: "test message to move",
	}
	_, err = h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Start move task
	startReq := &mockRequest{
		action:         types.ActionStartMessageMoveTask,
		sourceArn:      srcArn,
		destinationArn: dstArn,
	}
	resp, err := h.HandleRequest(context.Background(), startReq, handlers.QueryProtocol)
	require.NoError(t, err)
	require.NotNil(t, resp.MoveTaskResult)
	assert.NotEmpty(t, resp.MoveTaskResult.TaskHandle)

	// Give the task time to run
	time.Sleep(200 * time.Millisecond)

	// Verify message was moved to destination
	recvReq := &mockRequest{
		action:              "ReceiveMessage",
		queueURL:            "http://localhost:9324/123456789012/move-dst",
		maxNumberOfMessages: 10,
	}
	recvResp, err := h.HandleRequest(context.Background(), recvReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Len(t, recvResp.Messages, 1)
	assert.Equal(t, "test message to move", recvResp.Messages[0].Body)
}

func TestStartMessageMoveTask_AutoDiscoverDestination(t *testing.T) {
	h := newTestHandler()

	// Create DLQ
	createDLQ := &mockRequest{action: "CreateQueue", queueName: "auto-dlq"}
	_, err := h.HandleRequest(context.Background(), createDLQ, handlers.QueryProtocol)
	require.NoError(t, err)

	// Get DLQ ARN
	dlqAttrsReq := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/auto-dlq",
		attributeNames: []string{"QueueArn"},
	}
	dlqAttrsResp, err := h.HandleRequest(context.Background(), dlqAttrsReq, handlers.QueryProtocol)
	require.NoError(t, err)
	dlqArn := dlqAttrsResp.Attributes["QueueArn"]
	require.NotEmpty(t, dlqArn)

	// Create main queue with redrive policy pointing to DLQ
	redrivePolicy := `{"deadLetterTargetArn":"` + dlqArn + `","maxReceiveCount":"3"}`
	createMain := &mockRequest{
		action:    "CreateQueue",
		queueName: "auto-main",
		attributes: map[string]string{
			"RedrivePolicy": redrivePolicy,
		},
	}
	_, err = h.HandleRequest(context.Background(), createMain, handlers.QueryProtocol)
	require.NoError(t, err)

	// Send a message to the DLQ
	sendReq := &mockRequest{
		action:      "SendMessage",
		queueURL:    "http://localhost:9324/123456789012/auto-dlq",
		messageBody: "message in DLQ",
	}
	_, err = h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
	require.NoError(t, err)

	// Start move task with empty destination (auto-discover)
	startReq := &mockRequest{
		action:         types.ActionStartMessageMoveTask,
		sourceArn:      dlqArn,
		destinationArn: "", // empty — should auto-discover
	}
	resp, err := h.HandleRequest(context.Background(), startReq, handlers.QueryProtocol)
	require.NoError(t, err)
	require.NotNil(t, resp.MoveTaskResult)
	assert.NotEmpty(t, resp.MoveTaskResult.TaskHandle)

	// Give the task time to run
	time.Sleep(200 * time.Millisecond)

	// Verify message was moved back to main queue
	recvReq := &mockRequest{
		action:              "ReceiveMessage",
		queueURL:            "http://localhost:9324/123456789012/auto-main",
		maxNumberOfMessages: 10,
	}
	recvResp, err := h.HandleRequest(context.Background(), recvReq, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Len(t, recvResp.Messages, 1)
	assert.Equal(t, "message in DLQ", recvResp.Messages[0].Body)
}

// ---------------------------------------------------------------------------
// CancelMessageMoveTask Tests
// ---------------------------------------------------------------------------

func TestCancelMessageMoveTask_MissingTaskHandle(t *testing.T) {
	h := newTestHandler()

	req := &mockRequest{
		action: types.ActionCancelMessageMoveTask,
	}

	_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TaskHandle")
}

func TestCancelMessageMoveTask_NotFound(t *testing.T) {
	h := newTestHandler()

	req := &mockRequest{
		action:     types.ActionCancelMessageMoveTask,
		taskHandle: "nonexistent-handle",
	}

	_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	assert.Error(t, err)
}

func TestCancelMessageMoveTask_Success(t *testing.T) {
	h := newTestHandler()

	// Create source and destination queues
	createSrc := &mockRequest{action: "CreateQueue", queueName: "cancel-src"}
	_, err := h.HandleRequest(context.Background(), createSrc, handlers.QueryProtocol)
	require.NoError(t, err)

	srcAttrsReq := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/cancel-src",
		attributeNames: []string{"QueueArn"},
	}
	srcAttrsResp, err := h.HandleRequest(context.Background(), srcAttrsReq, handlers.QueryProtocol)
	require.NoError(t, err)
	srcArn := srcAttrsResp.Attributes["QueueArn"]

	createDst := &mockRequest{action: "CreateQueue", queueName: "cancel-dst"}
	_, err = h.HandleRequest(context.Background(), createDst, handlers.QueryProtocol)
	require.NoError(t, err)

	dstAttrsReq := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/cancel-dst",
		attributeNames: []string{"QueueArn"},
	}
	dstAttrsResp, err := h.HandleRequest(context.Background(), dstAttrsReq, handlers.QueryProtocol)
	require.NoError(t, err)
	dstArn := dstAttrsResp.Attributes["QueueArn"]

	// Send multiple messages to source
	for i := 0; i < 5; i++ {
		sendReq := &mockRequest{
			action:      "SendMessage",
			queueURL:    "http://localhost:9324/123456789012/cancel-src",
			messageBody: "message " + string(rune('A'+i)),
		}
		_, err = h.HandleRequest(context.Background(), sendReq, handlers.QueryProtocol)
		require.NoError(t, err)
	}

	// Start move task with rate limiting (1 msg/sec) so we can cancel it
	startReq := &mockRequest{
		action:         types.ActionStartMessageMoveTask,
		sourceArn:      srcArn,
		destinationArn: dstArn,
		maxMoveRate:    1,
	}
	resp, err := h.HandleRequest(context.Background(), startReq, handlers.QueryProtocol)
	require.NoError(t, err)
	require.NotNil(t, resp.MoveTaskResult)
	taskHandle := resp.MoveTaskResult.TaskHandle
	require.NotEmpty(t, taskHandle)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Cancel the task
	cancelReq := &mockRequest{
		action:     types.ActionCancelMessageMoveTask,
		taskHandle: taskHandle,
	}
	cancelResp, err := h.HandleRequest(context.Background(), cancelReq, handlers.QueryProtocol)
	require.NoError(t, err)
	require.NotNil(t, cancelResp.MoveTaskResult)
	// MovedMessages should be >= 0
	assert.GreaterOrEqual(t, cancelResp.MoveTaskResult.MovedMessages, 0)
}

// ---------------------------------------------------------------------------
// ListMessageMoveTasks Tests
// ---------------------------------------------------------------------------

func TestListMessageMoveTasks_MissingSourceArn(t *testing.T) {
	h := newTestHandler()

	req := &mockRequest{
		action: types.ActionListMessageMoveTasks,
	}

	_, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SourceArn")
}

func TestListMessageMoveTasks_Empty(t *testing.T) {
	h := newTestHandler()

	req := &mockRequest{
		action:    types.ActionListMessageMoveTasks,
		sourceArn: "arn:aws:sqs:us-east-1:123456789012:no-tasks",
	}

	resp, err := h.HandleRequest(context.Background(), req, handlers.QueryProtocol)
	require.NoError(t, err)
	assert.Empty(t, resp.MoveTaskResults)
}

func TestListMessageMoveTasks_WithTask(t *testing.T) {
	h := newTestHandler()

	// Create source and destination
	createSrc := &mockRequest{action: "CreateQueue", queueName: "list-src"}
	_, err := h.HandleRequest(context.Background(), createSrc, handlers.QueryProtocol)
	require.NoError(t, err)

	srcAttrsReq := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/list-src",
		attributeNames: []string{"QueueArn"},
	}
	srcAttrsResp, err := h.HandleRequest(context.Background(), srcAttrsReq, handlers.QueryProtocol)
	require.NoError(t, err)
	srcArn := srcAttrsResp.Attributes["QueueArn"]

	createDst := &mockRequest{action: "CreateQueue", queueName: "list-dst"}
	_, err = h.HandleRequest(context.Background(), createDst, handlers.QueryProtocol)
	require.NoError(t, err)

	dstAttrsReq := &mockRequest{
		action:         "GetQueueAttributes",
		queueURL:       "http://localhost:9324/123456789012/list-dst",
		attributeNames: []string{"QueueArn"},
	}
	dstAttrsResp, err := h.HandleRequest(context.Background(), dstAttrsReq, handlers.QueryProtocol)
	require.NoError(t, err)
	dstArn := dstAttrsResp.Attributes["QueueArn"]

	// Start a move task
	startReq := &mockRequest{
		action:         types.ActionStartMessageMoveTask,
		sourceArn:      srcArn,
		destinationArn: dstArn,
	}
	startResp, err := h.HandleRequest(context.Background(), startReq, handlers.QueryProtocol)
	require.NoError(t, err)
	taskHandle := startResp.MoveTaskResult.TaskHandle

	// List tasks for this source ARN
	listReq := &mockRequest{
		action:    types.ActionListMessageMoveTasks,
		sourceArn: srcArn,
	}
	listResp, err := h.HandleRequest(context.Background(), listReq, handlers.QueryProtocol)
	require.NoError(t, err)
	require.Len(t, listResp.MoveTaskResults, 1)
	assert.Equal(t, taskHandle, listResp.MoveTaskResults[0].TaskHandle)
	assert.Equal(t, srcArn, listResp.MoveTaskResults[0].SourceArn)
	assert.Equal(t, dstArn, listResp.MoveTaskResults[0].DestinationArn)
}
