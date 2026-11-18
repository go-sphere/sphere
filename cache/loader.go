package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/log"
	"golang.org/x/sync/singleflight"
)

// ErrDecode wraps the codec error returned when a stored entry exists but
// cannot be decoded into the requested type (an older schema, or another type
// sharing the key). GetObject, GetJson, CodecCache.Get and CodecCache.GetDel
// return it joined with the codec's own error, so errors.Is(err, ErrDecode) and
// errors.As on the codec error type both work. The read-through loaders (GetEx,
// GetObjectEx, GetJsonEx) treat it as a miss.
var ErrDecode = errors.New("cache: undecodable entry")

func decodeError(err error) error {
	return fmt.Errorf("%w: %w", ErrDecode, err)
}

// ErrTTLCalculatorType is returned when a WithDynamicTTL calculator receives a
// value of a different type than its type parameter, or is nil. SetObject hands
// the calculator the original value rather than the encoded bytes, so a
// mismatch means the calculator was instantiated for the wrong type. The write
// is rejected instead of falling back to a plain Set, which would silently store
// an entry that never expires, and instead of panicking on a nil calculator.
var ErrTTLCalculatorType = errors.New("cache: dynamic TTL calculator value type mismatch")

// loadOptions holds configuration settings for cache operations including TTL, singleflight, and dynamic TTL calculation.
type loadOptions struct {
	hasTTL        bool
	expiration    time.Duration
	singleflight  *singleflight.Group
	ttlCalculator func(value any) (bool, time.Duration, error)
}

func newOptions(opts ...Option) *loadOptions {
	defaults := &loadOptions{
		expiration: -1,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(defaults)
		}
	}
	return defaults
}

// Option defines a functional option for configuring cache behavior.
type Option func(o *loadOptions)

// WithExpiration stores via SetWithTTL. A negative duration is rejected by
// the driver as ErrInvalidTTL.
func WithExpiration(expiration time.Duration) Option {
	return func(o *loadOptions) {
		o.hasTTL = true
		o.expiration = expiration
		o.ttlCalculator = nil
	}
}

// WithNeverExpire stores via Set, which never expires and clears any existing TTL.
func WithNeverExpire() Option {
	return func(o *loadOptions) {
		o.hasTTL = false
		o.expiration = -1
		o.ttlCalculator = nil
	}
}

// WithSingleflight enables singleflight to prevent duplicate concurrent cache loads for the same key.
// This helps reduce redundant work when multiple goroutines request the same uncached data simultaneously.
//
// A caller whose ctx ends while it waits returns ctx.Err() with found=false;
// the shared build and its cache write keep running for the other callers,
// the write under the starting caller's ctx without its cancellation.
//
// Calls are deduplicated by cache key alone, so a group must not be shared
// between loads of different value types (the merged result is asserted to T
// and panics on mismatch) or between different caches (a value built for one
// cache would be returned for the other). Use one group per cache and type.
func WithSingleflight(single *singleflight.Group) Option {
	return func(o *loadOptions) {
		o.singleflight = single
	}
}

// WithDynamicTTL allows setting a dynamic TTL based on the value type T.
// The calculator function should return a boolean indicating whether the TTL is set
// and the duration for which the value should be cached.
//
// T must match the type being stored, and calculator must not be nil. When the
// type does not match or the calculator is nil, the write fails with
// ErrTTLCalculatorType rather than panicking, so a miswired option cannot take
// down a caller's request path.
func WithDynamicTTL[T any](calculator func(value T) (bool, time.Duration)) Option {
	return func(o *loadOptions) {
		o.ttlCalculator = func(value any) (bool, time.Duration, error) {
			if calculator == nil {
				return false, 0, ErrTTLCalculatorType
			}
			typed, ok := value.(T)
			if !ok {
				return false, 0, ErrTTLCalculatorType
			}
			hasTTL, expiration := calculator(typed)
			return hasTTL, expiration, nil
		}
	}
}

// Set writes value through c, applying TTL options. WithDynamicTTL sees the
// original value, not encoded bytes.
func Set[T any](ctx context.Context, c ExpirableCache[T], key string, value T, options ...Option) error {
	return setValueWithOptions(ctx, c, key, value, value, newOptions(options...))
}

func setValueWithOptions[S, T any](ctx context.Context, c ExpirableCache[S], key string, storedValue S, ttlSource T, opts *loadOptions) error {
	if opts.ttlCalculator != nil {
		hasTTL, expiration, err := opts.ttlCalculator(ttlSource)
		if err != nil {
			return err
		}
		if hasTTL {
			return c.SetWithTTL(ctx, key, storedValue, expiration)
		}
		return c.Set(ctx, key, storedValue)
	}
	if opts.hasTTL {
		return c.SetWithTTL(ctx, key, storedValue, opts.expiration)
	}
	return c.Set(ctx, key, storedValue)
}

func setObjectWithOptions[T any](ctx context.Context, c ExpirableByteCache, e codec.Encoder, key string, value T, opts *loadOptions) error {
	if e == nil {
		return errors.New("cache: nil encoder")
	}
	if opts.ttlCalculator != nil {
		hasTTL, expiration, err := opts.ttlCalculator(value)
		if err != nil {
			return err
		}
		if hasTTL && expiration < 0 {
			return ErrInvalidTTL
		}
		data, err := e.Marshal(value)
		if err != nil {
			return err
		}
		if hasTTL {
			return c.SetWithTTL(ctx, key, data, expiration)
		}
		return c.Set(ctx, key, data)
	}
	if opts.hasTTL && opts.expiration < 0 {
		return ErrInvalidTTL
	}
	data, err := e.Marshal(value)
	if err != nil {
		return err
	}
	if opts.hasTTL {
		return c.SetWithTTL(ctx, key, data, opts.expiration)
	}
	return c.Set(ctx, key, data)
}

// SetObject stores a typed object in a byte cache by encoding it with the provided encoder.
// This is useful for storing structured data that needs to be serialized before caching.
func SetObject[T any](ctx context.Context, c ExpirableByteCache, e codec.Encoder, key string, value T, options ...Option) error {
	return setObjectWithOptions(ctx, c, e, key, value, newOptions(options...))
}

// SetJson stores a typed object in a byte cache by encoding it as JSON.
// This is a convenience function that uses JSON marshaling for serialization.
func SetJson[T any](ctx context.Context, c ExpirableByteCache, key string, value T, options ...Option) error {
	return SetObject[T](ctx, c, codec.EncoderFunc(json.Marshal), key, value, options...)
}

// GetObject retrieves and decodes a typed object from a byte cache using the provided decoder.
// Returns the decoded object, whether it was found, and any error that occurred during retrieval or decoding.
// A decode failure returns found=false and an error wrapping ErrDecode; the entry is left in place.
func GetObject[T any](ctx context.Context, c ExpirableByteCache, d codec.Decoder, key string) (T, bool, error) {
	var value T
	if d == nil {
		return value, false, errors.New("cache: nil decoder")
	}
	data, found, err := c.Get(ctx, key)
	if err != nil {
		return value, false, err
	}
	if !found {
		return value, false, nil
	}
	err = d.Unmarshal(data, &value)
	if err != nil {
		var zero T
		return zero, false, decodeError(err)
	}
	return value, true, nil
}

// GetJson retrieves and decodes a JSON-encoded object from a byte cache.
// This is a convenience function that uses JSON unmarshalling for deserialization.
func GetJson[T any](ctx context.Context, c ExpirableByteCache, key string) (T, bool, error) {
	return GetObject[T](ctx, c, codec.DecoderFunc(json.Unmarshal), key)
}

// FetchCached is a function type that defines a builder for fetching cached objects.
// It should return the object of type T and an error if any occurs during the fetching process.
// If the error is nil, it indicates that the object was successfully fetched or built. So a cache can be set.
// If the object is not found or cannot be built, it should return a zero value of type T and an error.
// Then the cache will not be set.
type FetchCached[T any] = func() (obj T, err error)

// GetEx returns a cached value, or calls builder on a miss and stores the
// result. A builder error is returned with found=false and nothing is stored.
// A failed cache write after a successful builder is ignored: GetEx still
// returns (obj, true, nil). A nil builder on a miss returns (zero, false, nil).
//
// A cached entry that the getter reports as undecodable (an error wrapping
// ErrDecode) is logged at Warn level and treated as a miss: builder runs and
// its result overwrites the entry. With a nil builder nothing can replace the
// entry, so the decode error is returned instead. Any other getter error is
// returned without calling builder.
func GetEx[T any](ctx context.Context, c ExpirableCache[T], key string, builder FetchCached[T], options ...Option) (T, bool, error) {
	return load[T](
		ctx,
		key,
		c.Get,
		func(ctx context.Context, k string, v T, opts *loadOptions) error {
			return setValueWithOptions[T, T](ctx, c, k, v, v, opts)
		},
		builder,
		options...,
	)
}

// GetObjectEx is GetEx for a byte cache with an explicit decoder/encoder.
// Setter errors after a successful builder are ignored and an undecodable
// entry is rebuilt and overwritten, same as GetEx.
func GetObjectEx[T any](ctx context.Context, c ExpirableByteCache, d codec.Decoder, e codec.Encoder, key string, builder FetchCached[T], options ...Option) (T, bool, error) {
	return load[T](
		ctx,
		key,
		func(ctx context.Context, k string) (T, bool, error) {
			return GetObject[T](ctx, c, d, k)
		},
		func(ctx context.Context, k string, v T, opts *loadOptions) error {
			return setObjectWithOptions[T](ctx, c, e, k, v, opts)
		},
		builder,
		options...,
	)
}

// GetJsonEx is GetObjectEx with encoding/json.
func GetJsonEx[T any](ctx context.Context, c ExpirableByteCache, key string, builder FetchCached[T], options ...Option) (T, bool, error) {
	return GetObjectEx[T](ctx, c, codec.DecoderFunc(json.Unmarshal), codec.EncoderFunc(json.Marshal), key, builder, options...)
}

func load[T any](
	ctx context.Context,
	key string,
	getter func(context.Context, string) (T, bool, error),
	setter func(context.Context, string, T, *loadOptions) error,
	builder FetchCached[T],
	options ...Option,
) (T, bool, error) {
	opts := newOptions(options...)
	obj, found, gErr := getter(ctx, key)
	if gErr != nil {
		if !errors.Is(gErr, ErrDecode) || builder == nil {
			var zero T
			return zero, false, gErr
		}
		log.WarnContext(ctx, "cache: rebuilding undecodable entry",
			log.String("key", key),
			log.Err(gErr),
		)
		found = false
	}
	if found {
		return obj, true, nil
	}
	if builder == nil {
		var zero T
		return zero, false, nil
	}
	if opts.singleflight != nil {
		newObj, err := loadShared(ctx, opts.singleflight, key, setter, builder, opts)
		return newObj, err == nil, err
	}
	newObj, err := builder()
	if err == nil {
		_ = setter(ctx, key, newObj, opts)
	}
	return newObj, err == nil, err
}

// loadShared runs builder at most once per key across concurrent callers of
// group and stores its result. The shared build runs on its own goroutine and
// writes under ctx stripped of cancellation, so a caller whose ctx ends stops
// waiting with ctx.Err() while the build and write still complete for the
// callers that remain. A builder panic is re-raised in every waiting caller.
func loadShared[T any](
	ctx context.Context,
	group *singleflight.Group,
	key string,
	setter func(context.Context, string, T, *loadOptions) error,
	builder FetchCached[T],
	opts *loadOptions,
) (T, error) {
	var zero T
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	writeBase := context.WithoutCancel(ctx)
	deadline, hasDeadline := ctx.Deadline()
	ch := group.DoChan(key, func() (val any, err error) {
		// DoChan re-panics on a fresh goroutine, which no caller can recover,
		// so carry the panic back to the callers instead.
		defer func() {
			if r := recover(); r != nil {
				err = &builderPanic{value: r}
			}
		}()
		nObj, err := builder()
		if err == nil {
			writeCtx := writeBase
			if hasDeadline {
				var cancel context.CancelFunc
				writeCtx, cancel = context.WithDeadline(writeBase, deadline)
				defer cancel()
			}
			_ = setter(writeCtx, key, nObj, opts)
		}
		return nObj, err
	})
	select {
	case res := <-ch:
		if p, ok := res.Err.(*builderPanic); ok {
			panic(p.value)
		}
		if res.Err != nil {
			return zero, res.Err
		}
		// Preserve nil interface values (e.g. a nil error returned by builder),
		// while still rejecting a non-nil value from a mismatched type.
		if res.Val == nil {
			return zero, nil
		}
		obj, ok := res.Val.(T)
		if !ok {
			panic(fmt.Sprintf("cache: singleflight value type mismatch: got %T, want %T", res.Val, zero))
		}
		return cloneLoadedValue(obj), nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func cloneLoadedValue[T any](val T) T {
	if _, ok := any((*T)(nil)).(*[]byte); !ok {
		return val
	}
	raw, ok := any(val).([]byte)
	if !ok || raw == nil {
		return val
	}
	if cloned, ok := any(slices.Clone(raw)).(T); ok {
		return cloned
	}
	return val
}

// builderPanic carries a panic out of a shared build so it can be re-raised
// in the calling goroutines.
type builderPanic struct {
	value any
}

func (p *builderPanic) Error() string {
	return fmt.Sprintf("cache: builder panicked: %v", p.value)
}
