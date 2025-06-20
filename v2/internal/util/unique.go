package util

import (
	"fmt"
	"sync/atomic"
	"time"
)

var (
	idCounter uint64 = 0
)

// UniqueID creates a unique identifier for parsers by combining
// a nanosecond timestamp with an atomic counter to ensure uniqueness
func UniqueID(prefix string) string {
	// Get current time with nanosecond precision
	now := time.Now().UnixNano()

	// Atomically increment and get counter
	count := atomic.AddUint64(&idCounter, 1)

	// Combine them into a string
	return fmt.Sprintf("%s-%d-%d", prefix, now, count)
}
