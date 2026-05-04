package session

import (
	"encoding/base64"
	"sync"
	"testing"
)

func TestRingBuffer_Push(t *testing.T) {
	rb := NewRingBuffer(3)

	if seq := rb.Push([]byte("hello")); seq != 1 {
		t.Fatalf("expected seq 1, got %d", seq)
	}
	if seq := rb.Push([]byte("world")); seq != 2 {
		t.Fatalf("expected seq 2, got %d", seq)
	}

	if rb.Len() != 2 {
		t.Fatalf("expected len 2, got %d", rb.Len())
	}
	if rb.LatestSeq() != 2 {
		t.Fatalf("expected latest seq 2, got %d", rb.LatestSeq())
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := NewRingBuffer(2)

	rb.Push([]byte("a"))
	rb.Push([]byte("b"))
	rb.Push([]byte("c")) // should evict "a"

	if rb.Len() != 2 {
		t.Fatalf("expected len 2, got %d", rb.Len())
	}

	entries := rb.Since(1, 0)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// First entry should be "b" (seq 2), not "a" (evicted)
	decoded, _ := base64.StdEncoding.DecodeString(entries[0].Data)
	if string(decoded) != "b" {
		t.Fatalf("expected 'b', got '%s'", decoded)
	}
}

func TestRingBuffer_Since(t *testing.T) {
	rb := NewRingBuffer(10)

	rb.Push([]byte("a"))
	rb.Push([]byte("b"))
	rb.Push([]byte("c"))
	rb.Push([]byte("d"))

	entries := rb.Since(2, 0)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (since 2), got %d", len(entries))
	}

	entries = rb.Since(3, 1)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (since 3, limit 1), got %d", len(entries))
	}

	entries = rb.Since(99, 0)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (since 99), got %d", len(entries))
	}
}

func TestRingBuffer_Last(t *testing.T) {
	rb := NewRingBuffer(10)

	rb.Push([]byte("a"))
	rb.Push([]byte("b"))
	rb.Push([]byte("c"))

	entries := rb.Last(2)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	decoded, _ := base64.StdEncoding.DecodeString(entries[0].Data)
	if string(decoded) != "b" {
		t.Fatalf("expected 'b', got '%s'", decoded)
	}

	entries = rb.Last(10)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (all), got %d", len(entries))
	}

	entries = rb.Last(0)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (all with 0), got %d", len(entries))
	}
}

func TestRingBuffer_Clear(t *testing.T) {
	rb := NewRingBuffer(10)

	rb.Push([]byte("a"))
	rb.Push([]byte("b"))
	rb.Clear()

	if rb.Len() != 0 {
		t.Fatalf("expected len 0 after clear, got %d", rb.Len())
	}
	if rb.LatestSeq() != 0 {
		t.Fatalf("expected seq 0 after clear, got %d", rb.LatestSeq())
	}

	if seq := rb.Push([]byte("c")); seq != 1 {
		t.Fatalf("expected seq 1 after clear+push, got %d", seq)
	}
}

func TestRingBuffer_Concurrent(t *testing.T) {
	rb := NewRingBuffer(1000)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.Push([]byte("data"))
			}
		}()
	}

	wg.Wait()

	if rb.Len() != 1000 {
		t.Fatalf("expected 1000 entries, got %d", rb.Len())
	}
}

func TestRingBuffer_EmptyQueries(t *testing.T) {
	rb := NewRingBuffer(10)

	if entries := rb.Since(0, 10); len(entries) != 0 {
		t.Fatal("expected empty since on new buffer")
	}
	if entries := rb.Last(5); len(entries) != 0 {
		t.Fatal("expected empty last on new buffer")
	}
}
