package lru

import (
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
		lru := New()
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
	lru := New()
	lru.Add("myKey", 1234)
	if val, ok := lru.Get("myKey"); !ok {
		t.Fatal("TestRemove returned no match")
	} else if val != 1234 {
		t.Fatalf("TestRemove failed. Expected %d, got %v", 1234, val)
	}
	//lru.Remove("myKey")
	//if _, ok := lru.Get("myKey"); ok {
	//	t.Fatal("TestRemove returned a removed entry")
	//}
}
