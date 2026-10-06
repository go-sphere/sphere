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
//	return boot.NewApplication(httpServer, tracker), nil
//
// Online must be constructed with NewOnline; the zero value is unsupported:
// Start fails with ErrNotInitialized, the middleware fails requests with
// ErrNotInitialized, and Stop is a no-op that returns nil. Middleware and OnlineCount are safe for
// concurrent use.
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
// keygen runs before the downstream handler, so it sees only what earlier
// middleware stored (for example auth data). An empty key is not recorded. Each
// request refreshes its key's TTL; the request continues even if recording
// fails. Use a positive ttl: zero keeps keys until they are overwritten (never
// swept) and a negative ttl records nothing.
func (l *Online) Middleware(keygen func(ctx httpx.Context) string, ttl time.Duration) httpx.Middleware {
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			if l.cache == nil {
				// A zero-value Online has no backing cache; dereferencing it
				// panics on the request path. Start already fails fast with
				// ErrNotInitialized, so reaching a request through the zero
				// value means the tracker was never constructed with
				// NewOnline — fail the request with the same diagnosable
				// error.
				return ErrNotInitialized
			}
			key := keygen(ctx)
			if key != "" {
				_ = l.cache.SetWithTTL(ctx.Context(), key, struct{}{}, ttl)
			}
			return next(ctx)
		}
	}
}

// OnlineCount returns the number of keys currently resident in the tracker.
// The count is approximate: expired entries stay until Start's periodic Trim
// reclaims them, so a caller that polls this as a metric may briefly over-count.
// An Online built via its zero value tracks nothing, so the count is 0.
func (l *Online) OnlineCount() int {
	if l.cache == nil {
		return 0
	}
	return l.cache.Count()
}

// Identifier returns the task identifier for the online tracker.
func (l *Online) Identifier() string {
	return "online"
}

// Start runs the periodic sweep that reclaims expired entries. Reclaiming on a
// timer rather than inside the middleware keeps the sweep — which scans every
// key under the cache's write lock — off the request path.
//
// Start blocks until ctx is canceled (returning ctx.Err()) or Stop is called
// (returning nil). After Stop, Start returns nil immediately, so a tracker
// cannot be restarted.
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

// Stop ends the periodic sweep. It is idempotent, always returns nil, and may be
// called before Start or on a zero-value Online, whose Start already failed.
// Tracked keys stay readable after Stop, but expired keys are no longer
// reclaimed.
func (l *Online) Stop(ctx context.Context) error {
	if l.done == nil {
		// Zero value: Start returned ErrNotInitialized, so there is no sweep
		// to end, and closing the nil channel would panic.
		return nil
	}
	l.stopOnce.Do(func() {
		close(l.done)
	})
	return nil
}
