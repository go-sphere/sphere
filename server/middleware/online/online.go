package online

import (
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/mcache"
)

// Online tracks active users/sessions using a TTL-based cache.
// It maintains a count of online entities based on configurable key generation.
type Online struct {
	cache *mcache.Map[string, struct{}]
}

// NewOnline creates a new online tracking instance with an in-memory cache.
func NewOnline(cache cache.Cache[struct{}]) *Online {
	return &Online{
		cache: mcache.NewMapCache[struct{}](),
	}
}

// Middleware creates a Gin middleware that tracks online presence.
// It extracts a key from the request context and updates the online status with the specified TTL.
func (l *Online) Middleware(keygen func(ctx httpx.Context) string, ttl time.Duration) httpx.Middleware {
	return func(ctx httpx.Context) error {
		key := keygen(ctx)
		if key != "" {
			_ = l.cache.SetWithTTL(ctx, key, struct{}{}, ttl)
		}
		return ctx.Next()
	}
}

// OnlineCount returns the current number of online entities.
// This count reflects entries that have not yet expired from the cache.
func (l *Online) OnlineCount() int {
	return l.cache.Count()
}
