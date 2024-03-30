package epcache

import "github.com/EndlessParadox1/epcache/lru"

type cache struct {
	lru        *lru.Cache
	cacheBytes int64
}

func (c *cache) add(key string, value ByteView) {
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

func (c *cache) get(key string) (value ByteView, ok bool) {
	if c.lru == nil {
		return
	}
	if v, ok_ := c.lru.Get(key); ok_ {
		return v.(ByteView), true
	}
	return
}
