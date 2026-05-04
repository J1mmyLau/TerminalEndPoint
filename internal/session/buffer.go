package session

import (
	"encoding/base64"
	"sync"
)

// RingBuffer is a fixed-size circular buffer for storing terminal output
// with monotonically increasing sequence numbers.
type RingBuffer struct {
	mu     sync.RWMutex
	buf    []Entry
	size   int
	head   int
	tail   int
	count  int
	seq    uint64
}

// Entry represents a single buffered output line.
type Entry struct {
	Seq  uint64 `json:"seq"`
	Data string `json:"data"` // base64 encoded
}

// NewRingBuffer creates a ring buffer with the given capacity (number of entries).
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return &RingBuffer{
		buf:  make([]Entry, capacity),
		size: capacity,
	}
}

// Push adds a new entry to the buffer and returns its sequence number.
func (rb *RingBuffer) Push(data []byte) uint64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.seq++
	entry := Entry{
		Seq:  rb.seq,
		Data: base64.StdEncoding.EncodeToString(data),
	}

	rb.buf[rb.tail] = entry
	rb.tail = (rb.tail + 1) % rb.size

	if rb.count < rb.size {
		rb.count++
	} else {
		rb.head = (rb.head + 1) % rb.size
	}

	return rb.seq
}

// Since returns all entries with seq >= since, up to limit (0 = all).
func (rb *RingBuffer) Since(since uint64, limit int) []Entry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 || since > rb.seq {
		return nil
	}

	var result []Entry
	for i := 0; i < rb.count; i++ {
		idx := (rb.head + i) % rb.size
		e := rb.buf[idx]
		if e.Seq >= since {
			result = append(result, e)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// Last returns the most recent N entries.
func (rb *RingBuffer) Last(n int) []Entry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return nil
	}

	if n <= 0 || n > rb.count {
		n = rb.count
	}

	result := make([]Entry, 0, n)
	start := rb.count - n

	for i := 0; i < rb.count; i++ {
		idx := (rb.head + i) % rb.size
		if i >= start {
			result = append(result, rb.buf[idx])
		}
	}
	return result
}

// LatestSeq returns the most recent sequence number (0 if empty).
func (rb *RingBuffer) LatestSeq() uint64 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.seq
}

// Len returns the current number of entries in the buffer.
func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// Clear removes all entries and resets the sequence counter.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head = 0
	rb.tail = 0
	rb.count = 0
	rb.seq = 0
}
