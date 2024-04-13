package lru

import (
	"fmt"
	"testing"
)

var Tests = []struct {
	name       string
	keyToAdd   string
	keyToGet   string
	expectedOk bool
}{
	{"string_hit", "myKey", "myKey", true},
	{"string_miss", "myKey", "nonsense", false},
}

func TestGet(t *testing.T) {
	for _, tt := range Tests {
		lru := New(nil)
		lru.Add(tt.keyToAdd, 1234)
		val, ok := lru.Get(tt.keyToGet)
		if ok != tt.expectedOk {
			t.Fatalf("%s: cache hit = %v; want %v", tt.name, ok, !ok)
		} else if ok && val != 1234 {
			t.Fatalf("%s expected 1234 but got %v", tt.name, val)
		}
	}
}

func TestRemove(t *testing.T) {
	lru := New(nil)
	lru.Add("myKey", 1234)
	if val, ok := lru.Get("myKey"); !ok {
		t.Fatal("TestRemove returned no match")
	} else if val != 1234 {
		t.Fatalf("TestRemove failed. Expected %d, got %v", 1234, val)
	}
}

func TestEvict(t *testing.T) {
	var evictedKeys []string
	onEvictedFunc := func(key string, value any) {
		evictedKeys = append(evictedKeys, key)
	}
	lru := New(onEvictedFunc)
	for i := 0; i < 10; i++ {
		lru.Add(fmt.Sprintf("myKey%d", i), 1234)
	}
	lru.RemoveOldest()
	if evictedKeys[0] != "myKey0" {
		t.Fatalf("got evicted key: %s; want %s", evictedKeys[0], "myKey0")
	}
}
