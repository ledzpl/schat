package chat

import "sync"

// lineBuffer stores the user's current input line with concurrency protection.
type lineBuffer struct {
	mu   sync.RWMutex
	data []rune
}

func newLineBuffer(capacity int) *lineBuffer {
	if capacity <= 0 {
		capacity = 128
	}
	return &lineBuffer{
		data: make([]rune, 0, capacity),
	}
}

func (b *lineBuffer) Append(r rune) {
	b.mu.Lock()
	b.data = append(b.data, r)
	b.mu.Unlock()
}

func (b *lineBuffer) TrimLast() {
	b.mu.Lock()
	if n := len(b.data); n > 0 {
		b.data = b.data[:n-1]
	}
	b.mu.Unlock()
}

func (b *lineBuffer) Reset() {
	b.mu.Lock()
	b.data = b.data[:0]
	b.mu.Unlock()
}

func (b *lineBuffer) Drain() string {
	b.mu.Lock()
	text := string(b.data)
	b.data = b.data[:0]
	b.mu.Unlock()
	return text
}

func (b *lineBuffer) Snapshot() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return string(b.data)
}
