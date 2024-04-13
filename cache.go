package epcache

import (
	"github.com/EndlessParadox1/epcache/lru"
	"sync"
)

type cache struct {
	mu         sync.RWMutex
	nbytes     int
	lru        *lru.Cache
	nhit, nget int
	nevict     int
}

type CacheStats struct {
	Bytes  int
	Items  int
	Hits   int
	Gets   int
	Evicts int
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

func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		c.lru = lru.New(func(key string, value any) {
			c.nbytes -= len(key) + value.(ByteView).Len()
			c.nevict++
		})
	} // lazy init
	c.lru.Add(key, value)
	c.nbytes += len(key) + value.Len()
}

func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nget++
	if c.lru == nil {
		return
	}
	val, ok := c.lru.Get(key)
	if !ok {
		return
	}
	c.nhit++
	return val.(ByteView), true
}

func (c *cache) removeOldest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lru.RemoveOldest()
}

func (c *cache) items() int {
	if c.lru == nil {
		return 0
	}
	return c.lru.Len()
}
