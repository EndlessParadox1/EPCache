package lru

import (
	"reflect"
	"testing"
)

type String string

func (s String) Len() int {
	return len(s)
}

func TestGet(t *testing.T) {
	lru := New(int64(0), nil)
	lru.Add("key1", String("val1"))
	if v, ok := lru.Get("key1"); !ok || string(v.(String)) != "val1" {
		t.Fatal("cache hit key1=val1 failed")
	}
	if _, ok := lru.Get("key2"); ok {
		t.Fatal("cache miss key2 failed")
	}
}

func TestRemoveOldest(t *testing.T) {
	k1, k2, k3 := "key1", "key2", "key3"
	v1, v2, v3 := "value1", "value2", "value3"
	cap_ := len(k1 + k2 + v1 + v2)
	lru := New(int64(cap_), nil)
	lru.Add(k1, String(v1))
	lru.Add(k2, String(v2))
	lru.Add(k3, String(v3))
	if _, ok := lru.Get("key1"); ok || lru.Len() != 2 {
		t.Fatal("RemoveOldest key1 failed")
	}
}

func TestOnEvicted(t *testing.T) {
	var keys []string
	callback := func(key string, _ Value) {
		keys = append(keys, key)
	}
	lru := New(int64(10), callback)
	lru.Add("k1", String("v1"))
	lru.Add("k2", String("v2"))
	lru.Add("k3", String("v3"))
	lru.Add("k4", String("v4"))
	expect := []string{"k1", "k2"}
	if !reflect.DeepEqual(expect, keys) {
		t.Fatalf("Call OnEvicted failed, expect keys equals to %s", expect)
	}
}
