package app

import (
	"testing"
	"time"
)

func TestThrottle(t *testing.T) {
	now := time.Unix(1000, 0)
	th := NewThrottle(100*time.Millisecond, func() time.Time { return now })
	allowed := 0
	for i := 0; i < 100; i++ {
		if th.Allow() {
			allowed++
		}
		now = now.Add(10 * time.Millisecond)
	}
	if allowed > 10 || allowed < 1 {
		t.Fatalf("allowed=%d", allowed)
	}
}
