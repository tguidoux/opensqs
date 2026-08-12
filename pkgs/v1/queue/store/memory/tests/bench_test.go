package memory_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func newBenchStore(b *testing.B) *memory.MemoryStore {
	b.Helper()
	return memory.NewMemoryStore("bench-queue", 30, []byte("bench-secret"), store.StoreConfig{})
}

func newBenchMsg(i int) *types.Message {
	return &types.Message{
		MessageID: fmt.Sprintf("msg-%d", i),
		Body:      "benchmark message body",
		IsVisible: true,
	}
}

func BenchmarkSendMessage(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := s.SendMessage(ctx, newBenchMsg(i), 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReceiveMessage(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close()

	ctx := context.Background()

	// Pre-populate with b.N messages
	for i := 0; i < b.N; i++ {
		if err := s.SendMessage(ctx, newBenchMsg(i), 0); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msgs, err := s.ReceiveMessages(ctx, 1, 30, 0)
		if err != nil {
			b.Fatal(err)
		}
		if len(msgs) != 1 {
			b.Fatalf("expected 1 message, got %d", len(msgs))
		}
	}
}

func BenchmarkDeleteMessage(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close()

	ctx := context.Background()

	// Pre-populate with b.N messages
	for i := 0; i < b.N; i++ {
		if err := s.SendMessage(ctx, newBenchMsg(i), 0); err != nil {
			b.Fatal(err)
		}
	}

	// Receive all messages to get receipt handles.
	// ReceiveMessages returns max 10 per call and marks them invisible.
	// We use a large visibility timeout so handles don't expire.
	// Each call returns a different batch of 10 until all are received.
	handles := make([]string, 0, b.N)
	for len(handles) < b.N {
		msgs, err := s.ReceiveMessages(ctx, 10, 3600, 0)
		if err != nil {
			b.Fatal(err)
		}
		if len(msgs) == 0 {
			break
		}
		for _, m := range msgs {
			handles = append(handles, m.ReceiptHandle)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < len(handles); i++ {
		if err := s.DeleteMessage(ctx, handles[i]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSendMessageBatch10(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Send a batch of 10 messages
		for j := 0; j < 10; j++ {
			if err := s.SendMessage(ctx, newBenchMsg(i*10+j), 0); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkConcurrentSendReceive(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close()

	ctx := context.Background()

	// Pre-populate with b.N messages so the receiver always has work.
	for i := 0; i < b.N; i++ {
		if err := s.SendMessage(ctx, newBenchMsg(i), 0); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	wg.Add(2)

	// Sender goroutine — sends b.N additional messages
	go func() {
		defer wg.Done()
		for i := 0; i < b.N; i++ {
			if err := s.SendMessage(ctx, newBenchMsg(i+b.N), 0); err != nil {
				b.Error(err)
				return
			}
		}
	}()

	// Receiver goroutine — receives and deletes b.N messages
	go func() {
		defer wg.Done()
		for i := 0; i < b.N; i++ {
			msgs, err := s.ReceiveMessages(ctx, 1, 30, 0)
			if err != nil {
				b.Error(err)
				return
			}
			if len(msgs) > 0 {
				_ = s.DeleteMessage(ctx, msgs[0].ReceiptHandle)
			}
		}
	}()

	wg.Wait()
}
