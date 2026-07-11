package idgen

import (
	"container/list"
	"strconv"
	"sync"
	"time"
)

const defaultMaxKeys = 102400

type counter struct {
	ms  int64
	idx uint64
}

type entry struct {
	key     string
	counter counter
}

type lruCache struct {
	maxKeys int
	items   map[string]*list.Element
	order   *list.List
}

var (
	mu    sync.Mutex
	cache = newLRU(defaultMaxKeys)
)

func newLRU(maxKeys int) *lruCache {
	if maxKeys <= 0 {
		maxKeys = defaultMaxKeys
	}
	return &lruCache{
		maxKeys: maxKeys,
		items:   make(map[string]*list.Element, maxKeys),
		order:   list.New(),
	}
}

// SetMaxKeys configures the LRU capacity. Existing entries beyond the new limit
// are evicted immediately. Non-positive values fall back to defaultMaxKeys.
func SetMaxKeys(n int) {
	mu.Lock()
	defer mu.Unlock()
	cache.setMaxKeys(n)
}

func (c *lruCache) setMaxKeys(n int) {
	if n <= 0 {
		n = defaultMaxKeys
	}
	c.maxKeys = n
	for c.order.Len() > c.maxKeys {
		c.evictOldest()
	}
}

func (c *lruCache) evictOldest() {
	back := c.order.Back()
	if back == nil {
		return
	}
	e := back.Value.(*entry)
	delete(c.items, e.key)
	c.order.Remove(back)
}

// touch returns the counter for key, creating one if needed. The second return
// value is true when a new entry was inserted.
func (c *lruCache) touch(key string, nowMs int64) (*counter, bool) {
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		return &elem.Value.(*entry).counter, false
	}

	for c.order.Len() >= c.maxKeys {
		c.evictOldest()
	}

	e := &entry{key: key, counter: counter{ms: nowMs}}
	elem := c.order.PushFront(e)
	c.items[key] = elem
	return &e.counter, true
}

func (c *lruCache) len() int {
	return c.order.Len()
}

func formatID(ms int64, idx uint64) string {
	var buf [40]byte
	b := strconv.AppendInt(buf[:0], ms, 10)
	b = append(b, '-')
	b = strconv.AppendUint(b, idx, 10)
	return string(b)
}

// Get returns a unique id for the given key in the format "unixms-idx".
// unixms is the current unix timestamp in milliseconds; idx is a per-key
// sequence within that millisecond, starting at 0.
//
// Keys are tracked in a bounded LRU cache (default 102400 entries). When the
// cache is full, the least recently used key is evicted to cap memory growth.
func Get(key string) string {
	mu.Lock()
	defer mu.Unlock()

	nowMs := time.Now().UnixMilli()

	ct, created := cache.touch(key, nowMs)

	var idx uint64
	switch {
	case created:
		idx = 0
	case ct.ms != nowMs:
		ct.ms = nowMs
		ct.idx = 0
		idx = 0
	default:
		ct.idx++
		idx = ct.idx
	}

	return formatID(nowMs, idx)
}
