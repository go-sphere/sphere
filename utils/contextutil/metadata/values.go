package metadata

import (
	"context"
	"time"
)

var _ context.Context = (*values)(nil)

type values struct {
	context.Context
	data map[string]any
}

func WithValues(ctx context.Context, data map[string]any) context.Context {
	if len(data) == 0 {
		return ctx
	}
	return &values{
		Context: ctx,
		data:    data,
	}
}

func (c *values) Deadline() (deadline time.Time, ok bool) {
	return c.Context.Deadline()
}

func (c *values) Done() <-chan struct{} {
	return c.Context.Done()
}

func (c *values) Err() error {
	return c.Context.Err()
}

func (c *values) Value(key any) any {
	if strKey, ok := key.(string); ok {
		if v, exist := c.data[strKey]; exist {
			return v
		}
	}
	return c.Context.Value(key)
}
