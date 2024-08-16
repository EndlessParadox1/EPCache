// Package lru implements a lru cache.
package lru

import "container/list"

type Cache struct {
	ll    *list.List
	cache map[any]*list.Element
	// OnEvicted optionally specify a callback func to be executed when an entry is purged.
	onEvicted func(key string, value any)
}

type entry struct {
	key   string
	value any
}

func New(onEvicted func(string, any)) *Cache {
	return &Cache{
		ll:        list.New(),
		cache:     make(map[any]*list.Element),
		onEvicted: onEvicted,
	}
}

// Add adds or just updates an entry.
func (c *Cache) Add(key string, value any) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		ele.Value.(*entry).value = value
		return
	}
	ele := c.ll.PushFront(&entry{key, value})
	c.cache[key] = ele
}

func (c *Cache) Get(key string) (value any, ok bool) {
	if ele, hit := c.cache[key]; hit {
		c.ll.MoveToFront(ele)
		return ele.Value.(*entry).value, true
	}
	return
}

func (c *Cache) Remove(key string) {
	if ele, hit := c.cache[key]; hit {
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
	if c.onEvicted != nil {
		c.onEvicted(kv.key, kv.value)
	}
}

func (c *Cache) Len() int {
	return c.ll.Len()
}
