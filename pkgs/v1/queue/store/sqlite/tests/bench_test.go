package sqlite_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/tguidoux/opensqs/pkgs/v1/queue/store"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/store/sqlite"
	"github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func newBenchDB(b *testing.B) (*sql.DB, func()) {
	b.Helper()
	dbPath := fmt.Sprintf("/tmp/opensqs-bench-%d.db", time.Now().UnixNano())
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		b.Fatal(err)
	}
	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}
	return db, cleanup
}

func newBenchStore(b *testing.B) (*sqlite.SQLiteStore, func()) {
	b.Helper()
	db, cleanup := newBenchDB(b)
	s, err := sqlite.NewSQLiteStore(db, "bench-queue", 30, []byte("bench-secret"), store.StoreConfig{})
	if err != nil {
		cleanup()
		b.Fatal(err)
	}
	return s, func() {
		s.Close()
		cleanup()
	}
}

func newBenchMsg(i int) *types.Message {
	return &types.Message{
		MessageID: fmt.Sprintf("msg-%d", i),
		Body:      "benchmark message body",
		IsVisible: true,
	}
}

func BenchmarkSQLite_SendMessage(b *testing.B) {
	s, cleanup := newBenchStore(b)
	defer cleanup()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := s.SendMessage(ctx, newBenchMsg(i), 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSQLite_ReceiveMessage(b *testing.B) {
	s, cleanup := newBenchStore(b)
	defer cleanup()

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

func BenchmarkSQLite_DeleteMessage(b *testing.B) {
	s, cleanup := newBenchStore(b)
	defer cleanup()

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

func BenchmarkSQLite_SendMessageBatch10(b *testing.B) {
	s, cleanup := newBenchStore(b)
	defer cleanup()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			if err := s.SendMessage(ctx, newBenchMsg(i*10+j), 0); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkSQLite_ConcurrentSendReceive(b *testing.B) {
	s, cleanup := newBenchStore(b)
	defer cleanup()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	wg.Add(2)

	// Sender goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < b.N; i++ {
			if err := s.SendMessage(ctx, newBenchMsg(i), 0); err != nil {
				b.Error(err)
				return
			}
		}
	}()

	// Receiver goroutine
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
