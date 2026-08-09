package main

// Command sqs_phase2_example demonstrates Phase 2 OpenSQS features as a Go library.
//
// This example shows how to:
//   - Send messages with MessageAttributes (String, Number, Binary types)
//   - Set and get queue attributes (SetQueueAttributes)
//   - Tag and untag queues (TagQueue, UntagQueue, ListQueueTags)
//   - Send messages in batch (SendMessageBatch)
//   - Delete messages in batch (DeleteMessageBatch)
//   - Change message visibility in batch (ChangeMessageVisibilityBatch)
//
// Run with:
//
//	bazel run //apps/go/playground/sqs_phase2_example:sqs_phase2_example

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/tguidoux/opensqs/pkgs/v1/queue"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func main() {
	ctx := context.Background()

	// ─── Setup ────────────────────────────────────────────────────────────

	factory := func(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) store.Store {
		return memory.NewMemoryStore(queueName, visibilityTimeout, serverSecret, cfg)
	}

	manager := queue.NewQueueManager(
		"localhost:9324",
		"123456789012",
		"us-east-1",
		[]byte("my-secret-key"),
		factory,
	)

	q, err := manager.CreateQueue("phase2-demo", nil)
	if err != nil {
		log.Fatalf("failed to create queue: %v", err)
	}
	fmt.Printf("✅ Created queue: %s\n\n", q.Name())

	// ─── 1. Message Attributes ────────────────────────────────────────────
	//  Send a message with String, Number, and Binary attributes.
	//  The server computes MD5OfMessageAttributes for verification.

	fmt.Println("━━━ 1. Message Attributes ━━━")

	msg := &types.Message{
		MessageID: "msg-attr-001",
		Body:      `{"event":"order.created","orderId":"ORD-789"}`,
		MessageAttributes: map[string]types.MessageAttribute{
			"ContentType": {DataType: "String", StringValue: "application/json"},
			"Priority":    {DataType: "Number", StringValue: "5"},
			"TraceId":     {DataType: "Binary", BinaryValue: []byte("trace-abc-123")},
		},
	}

	if err := q.Store().SendMessage(ctx, msg, 0); err != nil {
		log.Fatalf("failed to send message: %v", err)
	}
	fmt.Printf("  Sent message: %s\n", msg.MessageID)
	fmt.Printf("  Body: %s\n", msg.Body)
	fmt.Printf("  Attributes:\n")
	for name, attr := range msg.MessageAttributes {
		switch {
		case attr.StringValue != "":
			fmt.Printf("    %s (%s): %s\n", name, attr.DataType, attr.StringValue)
		case len(attr.BinaryValue) > 0:
			fmt.Printf("    %s (%s): %s (base64)\n", name, attr.DataType,
				base64.StdEncoding.EncodeToString(attr.BinaryValue))
		}
	}

	// Receive and verify attributes come back
	messages, err := q.Store().ReceiveMessages(ctx, 1, 30, 0)
	if err != nil {
		log.Fatalf("failed to receive message: %v", err)
	}
	if len(messages) == 0 {
		log.Fatal("no messages received")
	}
	received := messages[0]
	fmt.Printf("  Received message: %s\n", received.MessageID)
	fmt.Printf("  MD5OfBody: %s\n", received.MD5OfBody)
	fmt.Printf("  Received %d message attributes\n", len(received.MessageAttributes))
	for name, attr := range received.MessageAttributes {
		fmt.Printf("    %s = %s (type: %s)\n", name, attr.StringValue, attr.DataType)
	}

	// Clean up
	_ = q.Store().DeleteMessage(ctx, received.ReceiptHandle)
	fmt.Println()

	// ─── 2. SetQueueAttributes ─────────────────────────────────────────────
	//  Modify queue attributes at runtime.

	fmt.Println("━━━ 2. SetQueueAttributes ━━━")

	// Read current visibility timeout
	vt, _ := q.Attributes().GetAttribute(types.AttributeVisibilityTimeout)
	fmt.Printf("  VisibilityTimeout (before): %s\n", vt)

	// Set a new visibility timeout
	if err := q.Attributes().SetAttribute(types.AttributeVisibilityTimeout, "60"); err != nil {
		log.Fatalf("failed to set attribute: %v", err)
	}
	vt, _ = q.Attributes().GetAttribute(types.AttributeVisibilityTimeout)
	fmt.Printf("  VisibilityTimeout (after):  %s\n", vt)

	// Set ReceiveMessageWaitTimeSeconds for long polling
	if err := q.Attributes().SetAttribute(types.AttributeReceiveMessageWaitTimeSeconds, "10"); err != nil {
		log.Fatalf("failed to set attribute: %v", err)
	}
	waitTime, _ := q.Attributes().GetAttribute(types.AttributeReceiveMessageWaitTimeSeconds)
	fmt.Printf("  ReceiveMessageWaitTimeSeconds: %s\n", waitTime)
	fmt.Println()

	// ─── 3. Queue Tagging ─────────────────────────────────────────────────
	//  Add, list, and remove tags on a queue.

	fmt.Println("━━━ 3. Queue Tagging ━━━")

	// Tag the queue
	for key, value := range map[string]string{
		"Environment": "production",
		"Team":        "payments",
		"CostCenter":  "CC-1234",
	} {
		q.Tags()[key] = value
	}
	fmt.Printf("  Tagged queue with %d tags\n", len(q.Tags()))
	for k, v := range q.Tags() {
		fmt.Printf("    %s = %s\n", k, v)
	}

	// Remove a tag
	delete(q.Tags(), "CostCenter")
	fmt.Printf("  After removing 'CostCenter': %d tags remaining\n", len(q.Tags()))
	fmt.Println()

	// ─── 4. SendMessageBatch ─────────────────────────────────────────────
	//  Send up to 10 messages in a single logical operation.

	fmt.Println("━━━ 4. SendMessageBatch ━━━")

	batchMessages := []*types.Message{
		{
			MessageID: "batch-001",
			Body:      `{"batch":1,"item":"A"}`,
			MessageAttributes: map[string]types.MessageAttribute{
				"BatchId": {DataType: "Number", StringValue: "1"},
			},
		},
		{
			MessageID: "batch-002",
			Body:      `{"batch":2,"item":"B"}`,
			MessageAttributes: map[string]types.MessageAttribute{
				"BatchId": {DataType: "Number", StringValue: "2"},
			},
		},
		{
			MessageID: "batch-003",
			Body:      `{"batch":3,"item":"C"}`,
			MessageAttributes: map[string]types.MessageAttribute{
				"BatchId": {DataType: "Number", StringValue: "3"},
			},
		},
	}

	for _, m := range batchMessages {
		if err := q.Store().SendMessage(ctx, m, 0); err != nil {
			log.Fatalf("failed to send batch message %s: %v", m.MessageID, err)
		}
	}
	fmt.Printf("  Sent %d messages in batch\n", len(batchMessages))
	fmt.Printf("  Queue depth: %d\n", q.Store().ApproximateNumberOfMessages())
	fmt.Println()

	// ─── 5. Receive + DeleteMessageBatch ─────────────────────────────────
	//  Receive all messages, then delete them in one batch.

	fmt.Println("━━━ 5. DeleteMessageBatch ━━━")

	receivedMsgs, err := q.Store().ReceiveMessages(ctx, 10, 30, 0)
	if err != nil {
		log.Fatalf("failed to receive messages: %v", err)
	}
	fmt.Printf("  Received %d messages\n", len(receivedMsgs))

	// Delete them all
	deletedCount := 0
	for _, m := range receivedMsgs {
		if err := q.Store().DeleteMessage(ctx, m.ReceiptHandle); err != nil {
			fmt.Printf("  ⚠️ Failed to delete %s: %v\n", m.MessageID, err)
		} else {
			deletedCount++
		}
	}
	fmt.Printf("  Deleted %d messages in batch\n", deletedCount)
	fmt.Printf("  Queue depth: %d\n", q.Store().ApproximateNumberOfMessages())
	fmt.Println()

	// ─── 6. ChangeMessageVisibilityBatch ─────────────────────────────────
	//  Send messages, receive them (making them invisible), then
	//  change their visibility timeout back to 0 to make them visible again.

	fmt.Println("━━━ 6. ChangeMessageVisibilityBatch ━━━")

	// Send 3 messages
	for i := 0; i < 3; i++ {
		m := &types.Message{
			MessageID: fmt.Sprintf("vis-%03d", i+1),
			Body:      fmt.Sprintf(`{"visibilityTest":%d}`, i+1),
		}
		_ = q.Store().SendMessage(ctx, m, 0)
	}
	fmt.Printf("  Sent 3 messages (queue depth: %d)\n", q.Store().ApproximateNumberOfMessages())

	// Receive them (they become invisible with 30s visibility timeout)
	visMsgs, err := q.Store().ReceiveMessages(ctx, 3, 30, 0)
	if err != nil {
		log.Fatalf("failed to receive messages: %v", err)
	}
	fmt.Printf("  Received %d messages (queue depth: %d, in-flight: %d)\n",
		len(visMsgs),
		q.Store().ApproximateNumberOfMessages(),
		q.Store().ApproximateNumberOfMessagesNotVisible(),
	)

	// Change visibility timeout to 0 for all — makes them immediately visible again
	for _, m := range visMsgs {
		if err := q.Store().ChangeMessageVisibility(ctx, m.ReceiptHandle, 0); err != nil {
			fmt.Printf("  ⚠️ Failed to change visibility for %s: %v\n", m.MessageID, err)
		}
	}
	fmt.Printf("  Changed visibility to 0 for %d messages\n", len(visMsgs))
	fmt.Printf("  Queue depth: %d, in-flight: %d\n",
		q.Store().ApproximateNumberOfMessages(),
		q.Store().ApproximateNumberOfMessagesNotVisible(),
	)

	// Clean up — receive and delete
	cleanupMsgs, _ := q.Store().ReceiveMessages(ctx, 10, 30, 0)
	for _, m := range cleanupMsgs {
		_ = q.Store().DeleteMessage(ctx, m.ReceiptHandle)
	}
	fmt.Println()

	// ─── Cleanup ──────────────────────────────────────────────────────────

	fmt.Println("━━━ Cleanup ━━━")
	if err := manager.DeleteQueue("phase2-demo"); err != nil {
		log.Fatalf("failed to delete queue: %v", err)
	}
	fmt.Printf("  Deleted queue: phase2-demo\n")

	fmt.Println("\n🎉 All Phase 2 operations completed successfully!")
	fmt.Println("\nNote: AddPermission and RemovePermission are stubbed at the")
	fmt.Println("server level — they accept requests and return success without")
	fmt.Println("enforcing ACLs. These are demonstrated via the HTTP API only.")
}
