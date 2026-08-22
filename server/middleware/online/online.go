package online

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache/mcache"
)

// defaultTrimInterval is how often Start reclaims expired entries.
const defaultTrimInterval = time.Minute

// ErrNotInitialized is returned by Start for a zero-value Online. The zero
// value has no backing cache and a zero trim interval, which would otherwise
// panic in time.NewTicker, so it fails the startup instead.
var ErrNotInitialized = errors.New("online: uninitialized Online: use NewOnline")

// Online tracks active users/sessions using a TTL-based cache.
// It maintains a count of online entities based on configurable key generation.
//
// Online implements core/task.Task and must be started for its storage to stay
// bounded. The backing cache reclaims an expired entry only when that key is
// read again or the whole map is swept, and the middleware only ever writes, so
// nothing reclaims anything on its own: with a high-cardinality key such as a
// client IP, session or device id, every key ever seen would otherwise stay
// resident for the life of the process. Return it from the application builder
// alongside the server:
//
//	tracker := online.NewOnline()
//	return []task.Task{tracker, httpServer}, nil
//
// Online must be constructed with NewOnline; the zero value is unsupported and
// Start fails with ErrNotInitialized instead of panicking.
type Online struct {
	cache        *mcache.Map[string, struct{}]
	trimInterval time.Duration

	done     chan struct{}
	stopOnce sync.Once
}

// Option configures an Online tracker.
type Option func(*Online)

// WithTrimInterval sets how often Start reclaims expired entries.
// A non-positive interval keeps the default.
func WithTrimInterval(interval time.Duration) Option {
	return func(o *Online) {
		if interval <= 0 {
			return
		}
		o.trimInterval = interval
	}
}

// NewOnline creates a new online tracking instance with an in-memory cache.
func NewOnline(options ...Option) *Online {
	o := &Online{
		cache:        mcache.NewMapCache[struct{}](),
		trimInterval: defaultTrimInterval,
		done:         make(chan struct{}),
	}
	for _, option := range options {
		if option != nil {
			option(o)
		}
	}
	return o
}

// Middleware creates a middleware that tracks online presence.
// It extracts a key from the request context and updates the online status with the specified TTL.
func (l *Online) Middleware(keygen func(ctx httpx.Context) string, ttl time.Duration) httpx.Middleware {
	return func(ctx httpx.Context) error {
		key := keygen(ctx)
		if key != "" {
			_ = l.cache.SetWithTTL(ctx.Context(), key, struct{}{}, ttl)
		}
		return ctx.Next()
	}
}

// OnlineCount returns the current number of online entities.
// This count reflects entries that have not yet expired from the cache.
func (l *Online) OnlineCount() int {
	return l.cache.Count()
}

// Identifier returns the task identifier for the online tracker.
func (l *Online) Identifier() string {
	return "online"
}

// Start runs the periodic sweep that reclaims expired entries. Reclaiming on a
// timer rather than inside the middleware keeps the sweep — which scans every
// key under the cache's write lock — off the request path.
func (l *Online) Start(ctx context.Context) error {
	if l.trimInterval <= 0 {
		return ErrNotInitialized
	}
	ticker := time.NewTicker(l.trimInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-l.done:
			return nil
		case <-ticker.C:
			l.cache.Trim()
		}
	}
}

// Stop ends the periodic sweep. It is idempotent.
func (l *Online) Stop(ctx context.Context) error {
	l.stopOnce.Do(func() {
		close(l.done)
	})
	return nil
}
