package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/tguidoux/opensqs/apps/go/server/handlers"
)

func BenchmarkHandleRequest_SendMessage(b *testing.B) {
	h := newTestHandler()

	// Create a queue for benchmarking
	createReq := &mockRequest{action: "CreateQueue", queueName: "bench-queue"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := &mockRequest{
			action:      "SendMessage",
			queueURL:    "http://localhost:9324/123456789012/bench-queue",
			messageBody: fmt.Sprintf("benchmark message %d", i),
		}
		_, err := h.HandleRequest(ctx, req, handlers.QueryProtocol)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHandleRequest_ReceiveMessage(b *testing.B) {
	h := newTestHandler()

	// Create a queue for benchmarking
	createReq := &mockRequest{action: "CreateQueue", queueName: "bench-queue"}
	_, err := h.HandleRequest(context.Background(), createReq, handlers.QueryProtocol)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	// Pre-populate with b.N messages
	for i := 0; i < b.N; i++ {
		sendReq := &mockRequest{
			action:      "SendMessage",
			queueURL:    "http://localhost:9324/123456789012/bench-queue",
			messageBody: fmt.Sprintf("benchmark message %d", i),
		}
		_, err := h.HandleRequest(ctx, sendReq, handlers.QueryProtocol)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := &mockRequest{
			action:              "ReceiveMessage",
			queueURL:            "http://localhost:9324/123456789012/bench-queue",
			maxNumberOfMessages: 1,
		}
		_, err := h.HandleRequest(ctx, req, handlers.QueryProtocol)
		if err != nil {
			b.Fatal(err)
		}
	}
}
