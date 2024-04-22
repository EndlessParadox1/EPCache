package epcache

import (
	"sync"
	"sync/atomic"

	"github.com/EndlessParadox1/epcache/lru"
)

type cache struct {
	mu         sync.RWMutex
	nbytes     int64
	lru        *lru.Cache
	nhit, nget int64
	nevict     int64
}

type CacheStats struct {
	Bytes  int64
	Items  int64
	Hits   int64
	Gets   int64
	Evicts int64
}

func (c *cache) stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheStats{
		Bytes:  c.nbytes,
		Items:  c.items(),
		Hits:   c.nhit,
		Gets:   c.nget,
		Evicts: c.nevict,
	}
}

func (c *cache) items() int64 {
	if c.lru == nil {
		return 0
	}
	return int64(c.lru.Len())
}

func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		c.lru = lru.New(func(key string, value any) {
			c.nbytes -= int64(len(key) + value.(ByteView).Len())
			c.nevict++
		})
	} // lazy init
	c.lru.Add(key, value)
	c.nbytes += int64(len(key) + value.Len())
}

func (c *cache) get(key string) (value ByteView, ok bool) {
	atomic.AddInt64(&c.nget, 1)
	if c.lru == nil {
		return
	}
	c.mu.RLock()
	val, ok_ := c.lru.Get(key)
	c.mu.RUnlock()
	if !ok_ {
		return
	}
	atomic.AddInt64(&c.nhit, 1)
	return val.(ByteView), true
}

func (c *cache) removeOldest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lru.RemoveOldest()
}

func (c *cache) bytes() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nbytes
}
