package integration

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_CreateQueueAndSendReceive tests the full SQS lifecycle:
// CreateQueue → SendMessage → ReceiveMessage → DeleteMessage
func TestIntegration_CreateQueueAndSendReceive(t *testing.T) {
	ts := newTestServer(t)

	// Step 1: Create a queue
	resp, body, err := ts.post("CreateQueue", "QueueName", "test-lifecycle-queue")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	queueURL := extractXMLValue(body, "QueueUrl")
	assert.Contains(t, queueURL, "test-lifecycle-queue")

	// Step 2: Send a message
	resp, body, err = ts.post("SendMessage",
		"QueueUrl", queueURL,
		"MessageBody", "Hello, integration test!",
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	messageID := extractXMLValue(body, "MessageId")
	assert.NotEmpty(t, messageID, "expected a MessageId in response")
	md5OfBody := extractXMLValue(body, "MD5OfMessageBody")
	assert.NotEmpty(t, md5OfBody, "expected MD5OfMessageBody in response")

	// Step 3: Receive the message
	resp, body, err = ts.post("ReceiveMessage",
		"QueueUrl", queueURL,
		"MaxNumberOfMessages", "1",
		"WaitTimeSeconds", "1",
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	receivedBody := extractXMLValue(body, "Body")
	assert.Equal(t, "Hello, integration test!", receivedBody)
	receiptHandle := extractXMLValue(body, "ReceiptHandle")
	assert.NotEmpty(t, receiptHandle, "expected a ReceiptHandle in response")

	// Step 4: Delete the message
	resp, body, err = ts.post("DeleteMessage",
		"QueueUrl", queueURL,
		"ReceiptHandle", receiptHandle,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Step 5: Verify queue is empty
	resp, body, err = ts.post("ReceiveMessage",
		"QueueUrl", queueURL,
		"MaxNumberOfMessages", "1",
		"WaitTimeSeconds", "0",
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// Should not contain a message body
	assert.NotContains(t, body, "Hello, integration test!")
}

// TestIntegration_SendMessageBatch tests sending multiple messages in a batch.
func TestIntegration_SendMessageBatch(t *testing.T) {
	ts := newTestServer(t)

	// Create a queue
	_, body, err := ts.post("CreateQueue", "QueueName", "test-batch-queue")
	require.NoError(t, err)
	queueURL := extractXMLValue(body, "QueueUrl")

	// Send a batch of messages using query protocol format
	form := url.Values{}
	form.Set("Action", "SendMessageBatch")
	form.Set("QueueUrl", queueURL)
	form.Set("SendMessageBatchRequestEntry.1.Id", "msg1")
	form.Set("SendMessageBatchRequestEntry.1.MessageBody", "First")
	form.Set("SendMessageBatchRequestEntry.2.Id", "msg2")
	form.Set("SendMessageBatchRequestEntry.2.MessageBody", "Second")
	form.Set("SendMessageBatchRequestEntry.3.Id", "msg3")
	form.Set("SendMessageBatchRequestEntry.3.MessageBody", "Third")

	resp, err := http.Post(ts.baseURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	body = string(bodyBytes)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Should have 3 successful entries
	entries := extractAllXMLValues(body, "SendMessageBatchResultEntry")
	assert.Len(t, entries, 3, "expected 3 batch result entries")

	// Receive all 3 messages
	for i := 0; i < 3; i++ {
		resp, body, err = ts.post("ReceiveMessage",
			"QueueUrl", queueURL,
			"MaxNumberOfMessages", "1",
			"WaitTimeSeconds", "1",
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		msgBody := extractXMLValue(body, "Body")
		assert.NotEmpty(t, msgBody)
	}
}

// TestIntegration_QueueAttributes tests setting and getting queue attributes.
func TestIntegration_QueueAttributes(t *testing.T) {
	ts := newTestServer(t)

	// Create a queue with custom attributes
	_, body, err := ts.post("CreateQueue",
		"QueueName", "test-attrs-queue",
		"Attribute.1.Name", "VisibilityTimeout",
		"Attribute.1.Value", "60",
		"Attribute.2.Name", "MessageRetentionPeriod",
		"Attribute.2.Value", "86400",
	)
	require.NoError(t, err)
	queueURL := extractXMLValue(body, "QueueUrl")

	// Get queue attributes
	resp, body, err := ts.post("GetQueueAttributes",
		"QueueUrl", queueURL,
		"AttributeName.1", "VisibilityTimeout",
		"AttributeName.2", "MessageRetentionPeriod",
		"AttributeName.3", "QueueArn",
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify VisibilityTimeout
	assert.Contains(t, body, "VisibilityTimeout")
	assert.Contains(t, body, "60")
	assert.Contains(t, body, "86400")
	assert.Contains(t, body, "arn:aws:sqs:us-east-1:123456789012:test-attrs-queue")
}

// TestIntegration_ListQueues tests listing queues with and without prefix.
func TestIntegration_ListQueues(t *testing.T) {
	ts := newTestServer(t)

	// Create multiple queues
	for _, name := range []string{"list-queue-a", "list-queue-b", "other-queue"} {
		_, _, err := ts.post("CreateQueue", "QueueName", name)
		require.NoError(t, err)
	}

	// List all queues
	resp, body, err := ts.post("ListQueues")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	urls := extractAllXMLValues(body, "QueueUrl")
	assert.GreaterOrEqual(t, len(urls), 3, "expected at least 3 queues in list")

	// List with prefix
	resp, body, err = ts.post("ListQueues", "Prefix", "list-")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	urls = extractAllXMLValues(body, "QueueUrl")
	assert.Len(t, urls, 2, "expected exactly 2 queues with prefix 'list-'")
}

// TestIntegration_DeleteQueue tests deleting a queue.
func TestIntegration_DeleteQueue(t *testing.T) {
	ts := newTestServer(t)

	// Create a queue
	_, body, err := ts.post("CreateQueue", "QueueName", "delete-me-queue")
	require.NoError(t, err)
	queueURL := extractXMLValue(body, "QueueUrl")

	// Delete the queue
	resp, _, err := ts.post("DeleteQueue", "QueueUrl", queueURL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify the queue is gone by trying to get its URL
	resp, body, err = ts.post("GetQueueUrl", "QueueName", "delete-me-queue")
	assert.NoError(t, err)
	// Should get an error response
	assert.True(t, resp.StatusCode >= 400 || strings.Contains(body, "QueueDoesNotExist"),
		"expected error when accessing deleted queue")
}

// TestIntegration_PurgeQueue tests purging all messages from a queue.
func TestIntegration_PurgeQueue(t *testing.T) {
	ts := newTestServer(t)

	// Create a queue
	_, body, err := ts.post("CreateQueue", "QueueName", "purge-test-queue")
	require.NoError(t, err)
	queueURL := extractXMLValue(body, "QueueUrl")

	// Send 3 messages
	for i := 0; i < 3; i++ {
		_, _, err := ts.post("SendMessage",
			"QueueUrl", queueURL,
			"MessageBody", fmt.Sprintf("message-%d", i),
		)
		require.NoError(t, err)
	}

	// Purge the queue
	resp, _, err := ts.post("PurgeQueue", "QueueUrl", queueURL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify queue is empty
	resp, body, err = ts.post("ReceiveMessage",
		"QueueUrl", queueURL,
		"MaxNumberOfMessages", "10",
		"WaitTimeSeconds", "0",
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, body, "message-0")
	assert.NotContains(t, body, "message-1")
	assert.NotContains(t, body, "message-2")
}

// TestIntegration_ChangeMessageVisibility tests changing the visibility timeout.
func TestIntegration_ChangeMessageVisibility(t *testing.T) {
	ts := newTestServer(t)

	// Create a queue with short visibility timeout
	_, body, err := ts.post("CreateQueue",
		"QueueName", "visibility-test-queue",
		"Attribute.1.Name", "VisibilityTimeout",
		"Attribute.1.Value", "2",
	)
	require.NoError(t, err)
	queueURL := extractXMLValue(body, "QueueUrl")

	// Send a message
	_, _, err = ts.post("SendMessage",
		"QueueUrl", queueURL,
		"MessageBody", "visibility-test",
	)
	require.NoError(t, err)

	// Receive the message
	_, body, err = ts.post("ReceiveMessage",
		"QueueUrl", queueURL,
		"MaxNumberOfMessages", "1",
		"WaitTimeSeconds", "1",
	)
	require.NoError(t, err)
	receiptHandle := extractXMLValue(body, "ReceiptHandle")
	require.NotEmpty(t, receiptHandle)

	// Change visibility timeout to 0 (make immediately visible)
	resp, _, err := ts.post("ChangeMessageVisibility",
		"QueueUrl", queueURL,
		"ReceiptHandle", receiptHandle,
		"VisibilityTimeout", "0",
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Should be able to receive the message again immediately
	resp, body, err = ts.post("ReceiveMessage",
		"QueueUrl", queueURL,
		"MaxNumberOfMessages", "1",
		"WaitTimeSeconds", "1",
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "visibility-test")
}

// TestIntegration_GetQueueURL tests the GetQueueUrl action.
func TestIntegration_GetQueueURL(t *testing.T) {
	ts := newTestServer(t)

	// Create a queue
	_, body, err := ts.post("CreateQueue", "QueueName", "url-test-queue")
	require.NoError(t, err)
	createdURL := extractXMLValue(body, "QueueUrl")

	// Get the queue URL
	resp, body, err := ts.post("GetQueueUrl", "QueueName", "url-test-queue")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	retrievedURL := extractXMLValue(body, "QueueUrl")
	assert.Equal(t, createdURL, retrievedURL)
}

// TestIntegration_SendMessageWithDelay tests delayed message delivery.
func TestIntegration_SendMessageWithDelay(t *testing.T) {
	ts := newTestServer(t)

	// Create a queue
	_, body, err := ts.post("CreateQueue", "QueueName", "delay-test-queue")
	require.NoError(t, err)
	queueURL := extractXMLValue(body, "QueueUrl")

	// Send a message with a 2-second delay
	_, _, err = ts.post("SendMessage",
		"QueueUrl", queueURL,
		"MessageBody", "delayed-message",
		"DelaySeconds", "2",
	)
	require.NoError(t, err)

	// Try to receive immediately — should be empty
	resp, body, err := ts.post("ReceiveMessage",
		"QueueUrl", queueURL,
		"MaxNumberOfMessages", "1",
		"WaitTimeSeconds", "0",
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, body, "delayed-message")

	// Wait for the delay to expire
	time.Sleep(3 * time.Second)

	// Now should receive the message
	resp, body, err = ts.post("ReceiveMessage",
		"QueueUrl", queueURL,
		"MaxNumberOfMessages", "1",
		"WaitTimeSeconds", "1",
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "delayed-message")
}

// TestIntegration_LongPolling tests that ReceiveMessage blocks until a message arrives.
func TestIntegration_LongPolling(t *testing.T) {
	ts := newTestServer(t)

	// Create a queue
	_, body, err := ts.post("CreateQueue", "QueueName", "longpoll-test-queue")
	require.NoError(t, err)
	queueURL := extractXMLValue(body, "QueueUrl")

	// Start a long-poll receive in a goroutine
	resultCh := make(chan string, 1)
	go func() {
		_, body, _ := ts.post("ReceiveMessage",
			"QueueUrl", queueURL,
			"MaxNumberOfMessages", "1",
			"WaitTimeSeconds", "5",
		)
		resultCh <- body
	}()

	// Wait a bit, then send a message
	time.Sleep(500 * time.Millisecond)
	_, _, err = ts.post("SendMessage",
		"QueueUrl", queueURL,
		"MessageBody", "long-poll-message",
	)
	require.NoError(t, err)

	// Wait for the receive to complete
	select {
	case body := <-resultCh:
		assert.Contains(t, body, "long-poll-message")
	case <-time.After(10 * time.Second):
		t.Fatal("long poll receive timed out")
	}
}

// TestIntegration_FIFOQueue tests FIFO queue creation and message ordering.
func TestIntegration_FIFOQueue(t *testing.T) {
	ts := newTestServer(t)

	// Create a FIFO queue
	_, body, err := ts.post("CreateQueue",
		"QueueName", "fifo-test-queue.fifo",
		"Attribute.1.Name", "FifoQueue",
		"Attribute.1.Value", "true",
		"Attribute.2.Name", "ContentBasedDeduplication",
		"Attribute.2.Value", "true",
	)
	require.NoError(t, err)
	queueURL := extractXMLValue(body, "QueueUrl")
	assert.Contains(t, queueURL, "fifo-test-queue.fifo")

	// Send messages with the same group ID
	for i := 0; i < 3; i++ {
		_, _, err := ts.post("SendMessage",
			"QueueUrl", queueURL,
			"MessageBody", fmt.Sprintf("fifo-msg-%d", i),
			"MessageGroupId", "group-1",
		)
		require.NoError(t, err)
	}

	// Receive messages — they should come in order for the same group
	for i := 0; i < 3; i++ {
		_, body, err := ts.post("ReceiveMessage",
			"QueueUrl", queueURL,
			"MaxNumberOfMessages", "1",
			"WaitTimeSeconds", "1",
		)
		require.NoError(t, err)
		msgBody := extractXMLValue(body, "Body")
		assert.Equal(t, fmt.Sprintf("fifo-msg-%d", i), msgBody,
			"messages should be received in FIFO order")

		// Delete the message to allow the next one
		receiptHandle := extractXMLValue(body, "ReceiptHandle")
		_, _, _ = ts.post("DeleteMessage",
			"QueueUrl", queueURL,
			"ReceiptHandle", receiptHandle,
		)
	}
}

// TestIntegration_FIFOQueueDeduplication tests content-based deduplication.
func TestIntegration_FIFOQueueDeduplication(t *testing.T) {
	ts := newTestServer(t)

	// Create a FIFO queue with content-based deduplication
	_, body, err := ts.post("CreateQueue",
		"QueueName", "dedup-test-queue.fifo",
		"Attribute.1.Name", "FifoQueue",
		"Attribute.1.Value", "true",
		"Attribute.2.Name", "ContentBasedDeduplication",
		"Attribute.2.Value", "true",
	)
	require.NoError(t, err)
	queueURL := extractXMLValue(body, "QueueUrl")

	// Send the same message twice
	for i := 0; i < 2; i++ {
		_, _, err := ts.post("SendMessage",
			"QueueUrl", queueURL,
			"MessageBody", "duplicate-message",
			"MessageGroupId", "group-1",
		)
		require.NoError(t, err)
	}

	// Should only receive one message (deduplication)
	_, body, err = ts.post("ReceiveMessage",
		"QueueUrl", queueURL,
		"MaxNumberOfMessages", "10",
		"WaitTimeSeconds", "1",
	)
	require.NoError(t, err)
	messages := extractAllXMLValues(body, "Body")
	assert.Len(t, messages, 1, "expected only 1 message due to deduplication")
	assert.Equal(t, "duplicate-message", messages[0])
}

// TestIntegration_InvalidQueueName tests error handling for invalid queue names.
func TestIntegration_InvalidQueueName(t *testing.T) {
	ts := newTestServer(t)

	// Try to create a queue with an empty name
	resp, _, err := ts.post("CreateQueue", "QueueName", "")
	require.NoError(t, err)
	assert.True(t, resp.StatusCode >= 400, "expected error for empty queue name")

	// Try to create a queue with invalid characters
	resp, _, err = ts.post("CreateQueue", "QueueName", "invalid/queue/name")
	require.NoError(t, err)
	assert.True(t, resp.StatusCode >= 400, "expected error for invalid queue name")
}

// TestIntegration_ReceiveFromNonExistentQueue tests receiving from a queue that doesn't exist.
func TestIntegration_ReceiveFromNonExistentQueue(t *testing.T) {
	ts := newTestServer(t)

	// With autoCreate=true, the queue will be created automatically.
	// So we test receiving from a non-existent queue URL instead.
	resp, body, err := ts.post("ReceiveMessage",
		"QueueUrl", "http://localhost:9324/123456789012/nonexistent-queue",
		"MaxNumberOfMessages", "1",
		"WaitTimeSeconds", "0",
	)
	require.NoError(t, err)
	// With autoCreate, the queue is created and ReceiveMessage returns an empty response.
	assert.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 OK, got body: %s", body)
}

// TestIntegration_MultipleQueuesIsolation tests that messages in different queues don't mix.
func TestIntegration_MultipleQueuesIsolation(t *testing.T) {
	ts := newTestServer(t)

	// Create two queues
	_, body, err := ts.post("CreateQueue", "QueueName", "isolation-queue-a")
	require.NoError(t, err)
	queueURLA := extractXMLValue(body, "QueueUrl")

	_, body, err = ts.post("CreateQueue", "QueueName", "isolation-queue-b")
	require.NoError(t, err)
	queueURLB := extractXMLValue(body, "QueueUrl")

	// Send a message to queue A
	_, _, err = ts.post("SendMessage",
		"QueueUrl", queueURLA,
		"MessageBody", "message-in-a",
	)
	require.NoError(t, err)

	// Send a message to queue B
	_, _, err = ts.post("SendMessage",
		"QueueUrl", queueURLB,
		"MessageBody", "message-in-b",
	)
	require.NoError(t, err)

	// Receive from queue A — should only get message-in-a
	_, body, err = ts.post("ReceiveMessage",
		"QueueUrl", queueURLA,
		"MaxNumberOfMessages", "10",
		"WaitTimeSeconds", "1",
	)
	require.NoError(t, err)
	assert.Contains(t, body, "message-in-a")
	assert.NotContains(t, body, "message-in-b")

	// Receive from queue B — should only get message-in-b
	_, body, err = ts.post("ReceiveMessage",
		"QueueUrl", queueURLB,
		"MaxNumberOfMessages", "10",
		"WaitTimeSeconds", "1",
	)
	require.NoError(t, err)
	assert.Contains(t, body, "message-in-b")
	assert.NotContains(t, body, "message-in-a")
}

// TestIntegration_MessageVisibilityTimeout tests that a received message becomes
// visible again after the visibility timeout expires.
func TestIntegration_MessageVisibilityTimeout(t *testing.T) {
	ts := newTestServer(t)

	// Create a queue with a 1-second visibility timeout
	_, body, err := ts.post("CreateQueue",
		"QueueName", "vt-test-queue",
		"Attribute.1.Name", "VisibilityTimeout",
		"Attribute.1.Value", "1",
	)
	require.NoError(t, err)
	queueURL := extractXMLValue(body, "QueueUrl")

	// Send a message
	_, _, err = ts.post("SendMessage",
		"QueueUrl", queueURL,
		"MessageBody", "vt-test-message",
	)
	require.NoError(t, err)

	// Receive the message
	_, body, err = ts.post("ReceiveMessage",
		"QueueUrl", queueURL,
		"MaxNumberOfMessages", "1",
		"WaitTimeSeconds", "1",
	)
	require.NoError(t, err)
	assert.Contains(t, body, "vt-test-message")

	// Try to receive again immediately — should be empty (message is invisible)
	_, body, err = ts.post("ReceiveMessage",
		"QueueUrl", queueURL,
		"MaxNumberOfMessages", "1",
		"WaitTimeSeconds", "0",
	)
	require.NoError(t, err)
	assert.NotContains(t, body, "vt-test-message")

	// Wait for visibility timeout to expire
	time.Sleep(2 * time.Second)

	// Should receive the message again
	_, body, err = ts.post("ReceiveMessage",
		"QueueUrl", queueURL,
		"MaxNumberOfMessages", "1",
		"WaitTimeSeconds", "1",
	)
	require.NoError(t, err)
	assert.Contains(t, body, "vt-test-message")
}
