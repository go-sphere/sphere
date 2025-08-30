package metadata

import (
	"context"
	"time"
)

var _ context.Context = (*values)(nil)

// values implements context.Context with additional key-value storage capabilities.
// It extends the parent context with a string-keyed data map for metadata storage.
type values struct {
	context.Context                // Parent context
	data            map[string]any // String-keyed metadata storage
}

// WithValues creates a new context with additional string-keyed metadata.
// The metadata is accessible through the Value method using string keys.
// Returns the original context if no data is provided.
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
