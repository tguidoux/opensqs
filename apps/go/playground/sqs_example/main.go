package main

// Command sqs_example demonstrates basic OpenSQS usage as a Go library.
//
// This example shows how to:
//   - Create a QueueManager
//   - Create a queue
//   - Send a message
//   - Receive a message
//   - Delete a message after processing
//   - List queues
//   - Purge and delete a queue
//
// Run with:
//
//	bazel run //apps/go/playground/sqs_example:sqs_example

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

	// 1. Create a QueueManager.
	//    The nodeAddress, accountID, and region are used to build queue URLs and ARNs.
	//    The serverSecret is used to sign receipt handles (use a strong secret in production).
	//    The storeFactory determines which Store implementation is used (memory, sqlite, etc.).
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

	// 2. Create a queue.
	//    Passing nil attributes uses SQS defaults (visibility timeout 30s, etc.).
	q, err := manager.CreateQueue("orders", nil)
	if err != nil {
		log.Fatalf("failed to create queue: %v", err)
	}
	fmt.Printf("✅ Created queue: %s\n", q.Name())
	fmt.Printf("   URL: %s\n", q.URL("localhost:9324", "123456789012"))
	fmt.Printf("   ARN: %s\n", q.ARN("us-east-1", "123456789012"))

	// 3. Send a message.
	msg := &types.Message{
		MessageID: "msg-001",
		Body:      `{"orderId":"12345","total":42.99,"currency":"USD"}`,
		MessageAttributes: map[string]types.MessageAttribute{
			"ContentType": {DataType: "String", StringValue: "application/json"},
			"Priority":    {DataType: "Number", StringValue: "1"},
		},
	}
	err = q.Store().SendMessage(ctx, msg, 0) // 0 = no delay
	if err != nil {
		log.Fatalf("failed to send message: %v", err)
	}
	fmt.Printf("✅ Sent message: %s\n", msg.MessageID)
	fmt.Printf("   Body: %s\n", msg.Body)

	// 4. Receive a message.
	//    maxMessages=1, visibilityTimeout=30s, waitTimeSeconds=0 (no long polling)
	messages, err := q.Store().ReceiveMessages(ctx, 1, 30, 0)
	if err != nil {
		log.Fatalf("failed to receive message: %v", err)
	}
	if len(messages) == 0 {
		log.Fatal("no messages received")
	}
	received := messages[0]
	fmt.Printf("✅ Received message: %s\n", received.MessageID)
	fmt.Printf("   Body: %s\n", received.Body)
	fmt.Printf("   ReceiptHandle: %s...\n", truncate(received.ReceiptHandle, 40))
	fmt.Printf("   ApproximateReceiveCount: %d\n", received.ApproximateReceiveCount)

	// 5. Process the message and delete it.
	//    In a real app, you'd do your business logic here.
	fmt.Println("   Processing message...")
	time.Sleep(100 * time.Millisecond)

	err = q.Store().DeleteMessage(ctx, received.ReceiptHandle)
	if err != nil {
		log.Fatalf("failed to delete message: %v", err)
	}
	fmt.Printf("✅ Deleted message: %s\n", received.MessageID)

	// 6. Send a few more messages and list queues.
	for i := 0; i < 3; i++ {
		m := &types.Message{
			MessageID: fmt.Sprintf("msg-%03d", i+2),
			Body:      fmt.Sprintf(`{"batch":%d}`, i),
		}
		_ = q.Store().SendMessage(ctx, m, 0)
	}
	fmt.Printf("✅ Sent 3 more messages (total in queue: %d)\n", q.Store().ApproximateNumberOfMessages())

	// List all queues.
	queues := manager.ListQueues("")
	fmt.Printf("✅ Listed queues (%d):\n", len(queues))
	for _, queue := range queues {
		fmt.Printf("   - %s\n", queue.Name())
	}

	// 7. Purge the queue (removes all messages).
	err = manager.PurgeQueue("orders")
	if err != nil {
		log.Fatalf("failed to purge queue: %v", err)
	}
	fmt.Printf("✅ Purged queue (messages remaining: %d)\n", q.Store().ApproximateNumberOfMessages())

	// 8. Delete the queue.
	err = manager.DeleteQueue("orders")
	if err != nil {
		log.Fatalf("failed to delete queue: %v", err)
	}
	fmt.Printf("✅ Deleted queue: orders\n")

	fmt.Println("\n🎉 All operations completed successfully!")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
