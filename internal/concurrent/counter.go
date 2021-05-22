package concurrent

import (
	"sync"
	"sync/atomic"
)

// Counter is a synchronous counter for tracking events and synchronising progress.
type Counter struct {
	waitGroup *sync.WaitGroup
	count     uint64
	vv        []interface{}
}

// NewCounter creates a new counter.
func NewCounter(waitGroup *sync.WaitGroup) *Counter {
	return &Counter{
		waitGroup: waitGroup,
		vv:        make([]interface{}, 0),
	}
}

// Track increments the counter by one and potentially adds the object to it's memory.
func (c *Counter) Track(v interface{}) {
	atomic.AddUint64(&c.count, 1)
	if v != nil {
		c.vv = append(c.vv, v)
	}
	if c.waitGroup != nil {
		c.waitGroup.Done()
	}
}

// Get returns the current count.
func (c *Counter) Get() int {
	return int(c.count)
}

// Values returns the tracked values.
func (c *Counter) Values() []interface{} {
	return c.vv
}
