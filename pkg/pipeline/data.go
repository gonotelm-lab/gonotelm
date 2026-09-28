package pipeline

import (
	"sync"
	"time"
)

type Data struct {
	mu    sync.RWMutex
	datas map[string]any
}

func NewData() *Data {
	return &Data{
		datas: make(map[string]any),
	}
}

func (c *Data) Get(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.datas[key]
}

func (c *Data) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.datas[key] = value
}

func (c *Data) GetBool(key string) bool {
	v, _ := c.Get(key).(bool)
	return v
}

func (c *Data) GetInt(key string) int {
	v, _ := c.Get(key).(int)
	return v
}

func (c *Data) GetInt64(key string) int64 {
	v, _ := c.Get(key).(int64)
	return v
}

func (c *Data) GetString(key string) string {
	v, _ := c.Get(key).(string)
	return v
}

func (c *Data) GetFloat64(key string) float64 {
	v, _ := c.Get(key).(float64)
	return v
}

func (c *Data) GetTime(key string) time.Time {
	v, _ := c.Get(key).(time.Time)
	return v
}

// Get returns the value stored at key as T. A missing key, or a value stored
// under a different type, yields the zero value of T.
func Get[T any](data *Data, key string) T {
	v, _ := data.Get(key).(T)
	return v
}
