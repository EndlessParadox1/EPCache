package consistenthash

import (
	"fmt"
	"strconv"
	"testing"
)

func TestHashing(t *testing.T) {
	hash := New(3, func(key []byte) uint32 {
		i, err := strconv.Atoi(string(key))
		if err != nil {
			panic(err)
		}
		return uint32(i)
	})
	// hash: 2, 4, 6, 12, 14, 16, 22, 24, 26
	hash.Add("6", "4", "2")
	testCases := map[string]string{
		"2":  "2",
		"11": "2",
		"23": "4",
		"27": "2",
	}
	for k, v := range testCases {
		if hash.Get(k) != v {
			t.Errorf("Asking for %s, should have yielded %s", k, v)
		}
	}
	// Adds 8, 18, 28
	hash.Add("8")
	// 27 should now map to 8.
	testCases["27"] = "8"
	for k, v := range testCases {
		if hash.Get(k) != v {
			t.Errorf("Asking for %s, should have yielded %s", k, v)
		}
	}
}

func TestConsistency(t *testing.T) {
	hash1 := New(1, nil)
	hash2 := New(1, nil)
	hash1.Add("Bill", "Bob", "Bonny")
	hash2.Add("Bob", "Bonny", "Bill")
	if hash1.Get("Ben") != hash2.Get("Ben") {
		t.Errorf("Fetching 'Ben' from both hashes should be the same")
	}
}

func BenchmarkGet8(b *testing.B) { benchmarkGet(b, 8) }

func BenchmarkGet32(b *testing.B) { benchmarkGet(b, 32) }

func BenchmarkGet128(b *testing.B) { benchmarkGet(b, 128) }

func BenchmarkGet512(b *testing.B) { benchmarkGet(b, 512) }

func benchmarkGet(b *testing.B, shards int) {
	hash := New(50, nil)
	var buckets []string
	for i := 0; i < shards; i++ {
		buckets = append(buckets, fmt.Sprintf("shard-%d", i))
	}
	hash.Add(buckets...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.Get(buckets[i&(shards-1)])
	}
}
