package msgctl

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	mc := New(2 * time.Second)
	go func() {
		for range 100 {
			mc.In <- struct{}{}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	var msgs []struct{}
	go func() {
		for range mc.Out {
			msgs = append(msgs, struct{}{})
		}
	}()
	time.Sleep(12 * time.Second)
	if len(msgs) != 6 {
		t.Fatalf("messages reduced into %d, want 6", len(msgs))
	}
}
