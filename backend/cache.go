package main

import (
	"container/list"
	"sync"
)

type lruCache[K comparable, V any] struct {
	mu      sync.Mutex
	maxSize int
	items   map[K]*list.Element
	order   *list.List
}

type cacheEntry[K comparable, V any] struct {
	key   K
	value V
}

func newLRUCache[K comparable, V any](maxSize int) *lruCache[K, V] {
	return &lruCache[K, V]{
		maxSize: maxSize,
		items:   make(map[K]*list.Element),
		order:   list.New(),
	}
}

func (c *lruCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		return elem.Value.(*cacheEntry[K, V]).value, true
	}
	var zero V
	return zero, false
}

func (c *lruCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		elem.Value.(*cacheEntry[K, V]).value = value
		return
	}

	if c.order.Len() >= c.maxSize {
		tail := c.order.Back()
		if tail != nil {
			entry := tail.Value.(*cacheEntry[K, V])
			delete(c.items, entry.key)
			c.order.Remove(tail)
		}
	}

	entry := &cacheEntry[K, V]{key: key, value: value}
	elem := c.order.PushFront(entry)
	c.items[key] = elem
}

func (c *lruCache[K, V]) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]*list.Element)
	c.order.Init()
}
