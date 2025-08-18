package metadata

import "context"

type metaKey struct{}

func WithMeta(ctx context.Context, m map[string]any) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

func MetaFrom(ctx context.Context) map[string]any {
	if v := ctx.Value(metaKey{}); v != nil {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}
