// Package bloomfilter avoids cache penetration.
package bloomfilter

import "github.com/spaolacci/murmur3"

// BloomFilter works as a blacklist, so it might refuse reqs for an existing key.
// Therefore, using it only when a large amount of non-existent keys will come to the group.
type BloomFilter struct {
	size   uint32
	bitmap []byte
	nhash  uint32
}

func New(size, nhash uint32) *BloomFilter {
	bitmapSize := (size + 7) / 8 // equal to ceil(size/8)
	return &BloomFilter{
		size:   size,
		bitmap: make([]byte, bitmapSize),
		nhash:  nhash,
	}
}

func (bf *BloomFilter) Add(key string) {
	for i := uint32(0); i < bf.nhash; i++ {
		index := bf.hash([]byte(key), i) % bf.size
		bf.bitmap[index/8] |= 1 << (index % 8)
	}
}

func (bf *BloomFilter) MightContain(key string) bool {
	for i := uint32(0); i < bf.nhash; i++ {
		index := bf.hash([]byte(key), i) % bf.size
		if bf.bitmap[index/8]&(1<<(index%8)) == 0 {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) hash(data []byte, seed uint32) uint32 {
	return murmur3.Sum32WithSeed(data, seed)
}
