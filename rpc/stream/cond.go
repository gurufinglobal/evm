package stream

import (
	"context"
	"sync"
)

// Cond implements conditional variable with a channel
type Cond struct {
	mu sync.Mutex // guards ch
	ch chan struct{}
}

func NewCond() *Cond {
	return &Cond{ch: make(chan struct{})}
}

// Wait returns true if the condition is signaled, false if the context is canceled
func (c *Cond) Wait(ctx context.Context) bool {
	return c.wait(ctx, nil)
}

// wait registers the notification before releasing the caller's condition lock.
func (c *Cond) wait(ctx context.Context, unlock func()) bool {
	c.mu.Lock()
	ch := c.ch
	c.mu.Unlock()
	if unlock != nil {
		unlock()
	}

	select {
	case <-ch:
		return true
	case <-ctx.Done():
		return false
	}
}

func (c *Cond) Broadcast() {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.ch)
	c.ch = make(chan struct{})
}
