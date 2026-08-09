package main

// Command sqs_fifo_example demonstrates FIFO queue features in OpenSQS.
//
// This example shows how to:
//   - Create a FIFO queue (must end with .fifo)
//   - Send messages with MessageGroupId and MessageDeduplicationId
//   - Receive messages in order within a message group
//   - Observe that only one message per group is in-flight at a time
//   - Use content-based deduplication
//   - See SequenceNumber in responses
//
// Run with:
//
//	bazel run //apps/go/playground/sqs_fifo_example:sqs_fifo_example

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

	// ─── 1. Create a FIFO Queue ───────────────────────────────────────────
	// FIFO queue names must end with ".fifo".
	// ContentBasedDeduplication uses the MD5 of the message body as the dedup ID.

	fmt.Println("━━━ 1. Create FIFO Queue ━━━")

	fifoAttrs := queue.NewDefaultQueueAttributes()
	fifoAttrs.FifoQueue = true
	fifoAttrs.ContentBasedDeduplication = true

	q, err := manager.CreateQueue("orders.fifo", fifoAttrs)
	if err != nil {
		log.Fatalf("failed to create FIFO queue: %v", err)
	}
	fmt.Printf("✅ Created FIFO queue: %s\n", q.Name())
	fmt.Printf("   IsFifo: %v\n", q.IsFifo())

	// ─── 2. Send Messages with Message Groups ─────────────────────────────
	// Messages within the same MessageGroupId are delivered in order.
	// Only one message per group is in-flight at a time.

	fmt.Println("\n━━━ 2. Send Messages with Message Groups ━━━")

	messages := []*types.Message{
		{
			MessageID:              "msg-001",
			Body:                   `{"order":"A","step":1}`,
			MessageGroupID:         "group-1",
			MessageDeduplicationID: "dedup-A-1",
		},
		{
			MessageID:              "msg-002",
			Body:                   `{"order":"A","step":2}`,
			MessageGroupID:         "group-1",
			MessageDeduplicationID: "dedup-A-2",
		},
		{
			MessageID:              "msg-003",
			Body:                   `{"order":"B","step":1}`,
			MessageGroupID:         "group-2",
			MessageDeduplicationID: "dedup-B-1",
		},
	}

	for _, msg := range messages {
		if err := q.Store().SendMessage(ctx, msg, 0); err != nil {
			log.Fatalf("failed to send message: %v", err)
		}
		fmt.Printf("✅ Sent: %s (group: %s, seq: %s)\n", msg.MessageID, msg.MessageGroupID, msg.SequenceNumber)
	}

	// ─── 3. Receive Messages — Ordered Within Groups ──────────────────────
	// The first receive returns one message per group (up to MaxNumberOfMessages).
	// group-1's first message and group-2's first message are both available.

	fmt.Println("\n━━━ 3. Receive Messages — Ordered Within Groups ━━━")

	received, err := q.Store().ReceiveMessages(ctx, 10, 30, 0)
	if err != nil {
		log.Fatalf("failed to receive messages: %v", err)
	}

	for _, msg := range received {
		fmt.Printf("✅ Received: %s (group: %s, body: %s)\n", msg.MessageID, msg.MessageGroupID, msg.Body)
	}

	// ─── 4. One In-Flight Per Group ───────────────────────────────────────
	// While group-1's first message is in-flight (not deleted),
	// the second message in group-1 is NOT available for receive.
	// But group-2's message was already received above, so the queue should be empty.

	fmt.Println("\n━━━ 4. One In-Flight Per Group ━━━")

	secondReceive, _ := q.Store().ReceiveMessages(ctx, 10, 30, 0)
	fmt.Printf("   Messages available (should be 0): %d\n", len(secondReceive))

	// ─── 5. Delete and Receive Next in Group ──────────────────────────────
	// After deleting the first message from group-1, the next message
	// in group-1 becomes available.

	fmt.Println("\n━━━ 5. Delete and Receive Next in Group ━━━")

	// Delete the first message from group-1
	for _, msg := range received {
		if msg.MessageGroupID == "group-1" {
			if err := q.Store().DeleteMessage(ctx, msg.ReceiptHandle); err != nil {
				log.Fatalf("failed to delete message: %v", err)
			}
			fmt.Printf("✅ Deleted: %s (group: %s)\n", msg.MessageID, msg.MessageGroupID)
			break
		}
	}

	// Now the second message from group-1 should be available
	nextMsgs, _ := q.Store().ReceiveMessages(ctx, 10, 30, 0)
	for _, msg := range nextMsgs {
		fmt.Printf("✅ Received next: %s (group: %s, body: %s)\n", msg.MessageID, msg.MessageGroupID, msg.Body)
	}

	// ─── 6. Content-Based Deduplication ───────────────────────────────────
	// With ContentBasedDeduplication=true, sending the same body within
	// the dedup window (5 minutes) is silently dropped.

	fmt.Println("\n━━━ 6. Content-Based Deduplication ━━━")

	// Purge the queue first
	_ = manager.PurgeQueue("orders.fifo")

	dedupMsg1 := &types.Message{
		MessageID:      "dedup-001",
		Body:           `{"event":"duplicate-test"}`,
		MessageGroupID: "group-1",
		// No MessageDeduplicationID — content-based dedup uses MD5 of body
	}
	dedupMsg2 := &types.Message{
		MessageID:      "dedup-002",
		Body:           `{"event":"duplicate-test"}`, // Same body!
		MessageGroupID: "group-1",
	}

	_ = q.Store().SendMessage(ctx, dedupMsg1, 0)
	_ = q.Store().SendMessage(ctx, dedupMsg2, 0) // Silently dropped (same content)

	fmt.Printf("✅ Sent 2 messages with identical body\n")
	fmt.Printf("   Queue depth (should be 1): %d\n", q.Store().ApproximateNumberOfMessages())

	// ─── Cleanup ──────────────────────────────────────────────────────────

	fmt.Println("\n━━━ Cleanup ━━━")
	_ = manager.PurgeQueue("orders.fifo")
	_ = manager.DeleteQueue("orders.fifo")
	fmt.Println("✅ Deleted FIFO queue")

	// Small delay to let visibility timers clean up
	time.Sleep(100 * time.Millisecond)
}
