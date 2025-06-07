package telegram

import (
	"context"
	"sync"
	"time"
)

var _ context.Context = &Context{}

type Context struct {
	ctx  context.Context
	mu   sync.RWMutex
	keys map[string]any
}

func NewContext(ctx context.Context) *Context {
	if ctx == nil {
		ctx = context.Background()
	}
	c := &Context{
		ctx:  ctx,
		keys: make(map[string]any, 3),
	}
	return c
}

func (c *Context) Deadline() (deadline time.Time, ok bool) {
	if c.ctx == nil {
		return time.Time{}, false
	}
	return c.ctx.Deadline()
}

func (c *Context) Done() <-chan struct{} {
	if c.ctx == nil {
		return nil
	}
	return c.ctx.Done()
}

func (c *Context) Err() error {
	if c.ctx == nil {
		return nil
	}
	return c.ctx.Err()
}

func (c *Context) Value(key any) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if strKey, ok := key.(string); ok {
		if v, exist := c.keys[strKey]; exist {
			return v
		}
	}
	if c.ctx == nil {
		return nil
	}
	return c.ctx.Value(key)
}

func (c *Context) SetValue(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys[key] = value
}
