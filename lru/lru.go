// Package lru implements a lru cache.
package lru

import "container/list"

type Cache struct {
	maxEntries int // 0 for no limit
	ll         *list.List
	cache      map[any]*list.Element
	// OnEvicted optionally specify a callback func to be executed when an entry is purged.
	OnEvicted func(key any, value any)
}

type entry struct {
	key   any
	value any
}

func New(maxEntries int) *Cache {
	return &Cache{
		maxEntries: maxEntries,
		ll:         list.New(),
		cache:      make(map[any]*list.Element),
	}
}

// Add adds or just updates an entry.
func (c *Cache) Add(key any, value any) {
	if c.cache == nil {
		c.cache = make(map[any]*list.Element)
		c.ll = list.New()
	}
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		ele.Value.(*entry).value = value
		return
	}
	ele := c.ll.PushFront(&entry{key, value})
	c.cache[key] = ele
	if c.maxEntries != 0 && c.ll.Len() > c.maxEntries {
		c.RemoveOldest()
	}
}

func (c *Cache) Get(key any) (value any, ok bool) {
	if c.cache == nil {
		return
	}
	if ele, hit := c.cache[key]; hit {
		c.ll.MoveToFront(ele)
		return ele.Value.(*entry).value, true
	}
	return
}

func (c *Cache) Remove(key any) {
	if c.cache == nil {
		return
	}
	if ele, ok := c.cache[key]; ok {
		c.removeElement(ele)
	}
}

func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.removeElement(ele)
	}
}

func (c *Cache) removeElement(ele *list.Element) {
	kv := c.ll.Remove(ele).(*entry)
	delete(c.cache, kv.key)
	if c.OnEvicted != nil {
		c.OnEvicted(kv.key, kv.value)
	}
}

func (c *Cache) Len() int {
	if c.cache == nil {
		return 0
	}
	return c.ll.Len()
}

// Clear purges all entries.
func (c *Cache) Clear() {
	if c.OnEvicted != nil {
		for _, ele := range c.cache {
			kv := ele.Value.(*entry)
			c.OnEvicted(kv.key, kv.value)
		}
	}
	c.cache = nil
	c.ll = nil
}
