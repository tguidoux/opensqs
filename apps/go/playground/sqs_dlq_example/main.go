package main

// Command sqs_dlq_example demonstrates Dead-Letter Queue (DLQ) features in OpenSQS.
//
// This example shows how to:
//   - Create a dead-letter queue
//   - Configure a RedrivePolicy on a main queue
//   - Send and receive messages
//   - Observe messages being redrived to the DLQ after maxReceiveCount
//   - List dead-letter source queues
//
// Run with:
//
//	bazel run //apps/go/playground/sqs_dlq_example:sqs_dlq_example

import (
	"context"
	"fmt"
	"log"
	"time"

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

	// ─── 1. Create the Dead-Letter Queue ──────────────────────────────────

	fmt.Println("━━━ 1. Create Dead-Letter Queue ━━━")

	dlq, err := manager.CreateQueue("failed-orders", nil)
	if err != nil {
		log.Fatalf("failed to create DLQ: %v", err)
	}
	dlqArn := dlq.ARN("us-east-1", "123456789012")
	fmt.Printf("✅ Created DLQ: %s\n", dlq.Name())
	fmt.Printf("   ARN: %s\n", dlqArn)

	// ─── 2. Create Main Queue with RedrivePolicy ──────────────────────────
	// RedrivePolicy is a JSON string with:
	//   - deadLetterTargetArn: ARN of the DLQ
	//   - maxReceiveCount: how many receive attempts before redrive

	fmt.Println("\n━━━ 2. Create Main Queue with RedrivePolicy ━━━")

	mainAttrs := queue.NewDefaultQueueAttributes()
	mainAttrs.RedrivePolicy = fmt.Sprintf(
		`{"deadLetterTargetArn":"%s","maxReceiveCount":"3"}`,
		dlqArn,
	)
	// Use a short visibility timeout so messages become visible again quickly
	mainAttrs.VisibilityTimeout = 1

	mainQueue, err := manager.CreateQueue("orders", mainAttrs)
	if err != nil {
		log.Fatalf("failed to create main queue: %v", err)
	}
	fmt.Printf("✅ Created main queue: %s\n", mainQueue.Name())
	fmt.Printf("   RedrivePolicy: %s\n", mainAttrs.RedrivePolicy)

	// ─── 3. Send a Message ────────────────────────────────────────────────

	fmt.Println("\n━━━ 3. Send a Message ━━━")

	msg := &types.Message{
		MessageID: "msg-001",
		Body:      `{"orderId":"12345","status":"pending"}`,
	}
	if err := mainQueue.Store().SendMessage(ctx, msg, 0); err != nil {
		log.Fatalf("failed to send message: %v", err)
	}
	fmt.Printf("✅ Sent: %s (body: %s)\n", msg.MessageID, msg.Body)

	// ─── 4. Receive Without Deleting — Trigger Redrive ────────────────────
	// Receive the message 3 times without deleting it.
	// After the 3rd receive, the message is redrived to the DLQ.

	fmt.Println("\n━━━ 4. Receive Without Deleting — Trigger Redrive ━━━")
	fmt.Println("   (maxReceiveCount=3, visibilityTimeout=1s)")

	for attempt := 1; attempt <= 3; attempt++ {
		// Wait for visibility timeout to expire so the message becomes visible again
		if attempt > 1 {
			fmt.Printf("   Waiting for visibility timeout to expire...\n")
			time.Sleep(1500 * time.Millisecond)
		}

		received, err := mainQueue.Store().ReceiveMessages(ctx, 1, 1, 0)
		if err != nil {
			log.Fatalf("failed to receive message: %v", err)
		}

		if len(received) == 0 {
			fmt.Printf("   Attempt %d: no messages available\n", attempt)
			continue
		}

		m := received[0]
		fmt.Printf("   Attempt %d: received %s (receiveCount: %d)\n", attempt, m.MessageID, m.ApproximateReceiveCount)

		// Don't delete — let visibility timeout expire so it becomes available again
	}

	// After 3rd receive, the message's visibility timer will fire and
	// trigger the redrive callback. Wait for that to complete.
	fmt.Println("   Waiting for final visibility timeout to expire and trigger redrive...")
	time.Sleep(1500 * time.Millisecond)

	// ─── 5. Verify Message Was Redrived to DLQ ───────────────────────────

	fmt.Println("\n━━━ 5. Verify Message Was Redrived to DLQ ━━━")

	// Main queue should be empty
	mainCount := mainQueue.Store().ApproximateNumberOfMessages()
	fmt.Printf("   Main queue depth (should be 0): %d\n", mainCount)

	// DLQ should have the message
	dlqMsgs, err := dlq.Store().ReceiveMessages(ctx, 10, 30, 0)
	if err != nil {
		log.Fatalf("failed to receive from DLQ: %v", err)
	}

	if len(dlqMsgs) == 0 {
		fmt.Println("   ⚠️  No messages in DLQ (redrive may not have triggered yet)")
	} else {
		for _, m := range dlqMsgs {
			fmt.Printf("✅ Found in DLQ: %s (body: %s)\n", m.MessageID, m.Body)
		}
	}

	// ─── 6. List Dead-Letter Source Queues ━───────────────────────────────
	// At the library level, we can check which queues have a RedrivePolicy
	// pointing to our DLQ by inspecting queue attributes.

	fmt.Println("\n━━━ 6. List Dead-Letter Source Queues ━━━")

	allQueues := manager.ListQueues("")
	for _, q := range allQueues {
		rp := q.Attributes().RedrivePolicy
		if rp != "" {
			fmt.Printf("✅ Queue '%s' has a RedrivePolicy\n", q.Name())
		}
	}

	// ─── Cleanup ──────────────────────────────────────────────────────────

	fmt.Println("\n━━━ Cleanup ━━━")
	_ = manager.PurgeQueue("orders")
	_ = manager.PurgeQueue("failed-orders")
	_ = manager.DeleteQueue("orders")
	_ = manager.DeleteQueue("failed-orders")
	fmt.Println("✅ Deleted queues")

	// Small delay to let visibility timers clean up
	time.Sleep(100 * time.Millisecond)
}
