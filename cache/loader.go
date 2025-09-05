package cache

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/go-sphere/confstore/codec"
	"golang.org/x/sync/singleflight"
)

// IsZero checks whether a value is the zero value of its type using deep comparison.
func IsZero[T any](t T) bool {
	var zero T
	return reflect.DeepEqual(t, zero)
}

// options holds configuration settings for cache operations including TTL, singleflight, and dynamic TTL calculation.
type options struct {
	hasTTL        bool
	expiration    time.Duration
	singleflight  *singleflight.Group
	ttlCalculator func(value any) (bool, time.Duration)
}

func newOptions(opts ...Option) *options {
	defaults := &options{
		hasTTL:       false,
		expiration:   -1,
		singleflight: nil,
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

// Option defines a functional option for configuring cache behavior.
type Option func(o *options)

// WithExpiration sets a fixed expiration duration for cache entries.
func WithExpiration(expiration time.Duration) Option {
	return func(o *options) {
		o.hasTTL = true
		o.expiration = expiration
	}
}

// WithNeverExpire configures the cache to never expire entries automatically.
func WithNeverExpire() Option {
	return func(o *options) {
		o.hasTTL = false
		o.expiration = -1
	}
}

// WithSingleflight enables singleflight to prevent duplicate concurrent cache loads for the same key.
// This helps reduce redundant work when multiple goroutines request the same uncached data simultaneously.
func WithSingleflight(single *singleflight.Group) Option {
	return func(o *options) {
		o.singleflight = single
	}
}

// WithDynamicTTL allows setting a dynamic TTL based on the value type T.
// The calculator function should return a boolean indicating whether the TTL is set,
// and the duration for which the value should be cached.
func WithDynamicTTL[T any](calculator func(value T) (bool, time.Duration)) Option {
	return func(o *options) {
		o.ttlCalculator = func(value any) (bool, time.Duration) {
			return calculator(value.(T))
		}
	}
}

// Set stores a value in the cache with optional configuration such as TTL.
// It applies the provided options to determine cache behavior like expiration and dynamic TTL calculation.
func Set[T any](ctx context.Context, c Cache[T], key string, value T, options ...Option) error {
	opts := newOptions(options...)
	if opts.ttlCalculator != nil {
		opts.hasTTL, opts.expiration = opts.ttlCalculator(value)
	}
	if opts.hasTTL {
		return c.SetWithTTL(ctx, key, value, opts.expiration)
	} else {
		return c.Set(ctx, key, value)
	}
}

// SetObject stores a typed object in a ByteCache by encoding it with the provided encoder.
// This is useful for storing structured data that needs to be serialized before caching.
func SetObject[T any, E codec.Encoder](ctx context.Context, c ByteCache, e E, key string, value T, options ...Option) error {
	data, err := e.Marshal(value)
	if err != nil {
		return err
	}
	return Set(ctx, c, key, data, options...)
}

// SetJson stores a typed object in a ByteCache by encoding it as JSON.
// This is a convenience function that uses JSON marshaling for serialization.
func SetJson[T any](ctx context.Context, c ByteCache, key string, value T, options ...Option) error {
	return SetObject[T, codec.EncoderFunc](ctx, c, json.Marshal, key, value, options...)
}

// GetObject retrieves and decodes a typed object from a ByteCache using the provided decoder.
// Returns the decoded object, whether it was found, and any error that occurred during retrieval or decoding.
func GetObject[T any, D codec.Decoder](ctx context.Context, c ByteCache, d D, key string) (T, bool, error) {
	data, found, err := c.Get(ctx, key)
	var value T
	if err != nil {
		return value, false, err
	}
	if !found {
		return value, false, nil
	}
	err = d.Unmarshal(data, &value)
	if err != nil {
		return value, false, err
	}
	return value, true, nil
}

// GetJson retrieves and decodes a JSON-encoded object from a ByteCache.
// This is a convenience function that uses JSON unmarshaling for deserialization.
func GetJson[T any](ctx context.Context, c ByteCache, key string, options ...Option) (T, bool, error) {
	return GetObject[T, codec.DecoderFunc](ctx, c, json.Unmarshal, key)
}

// FetchCached is a function type that defines a builder for fetching cached objects.
// It should return the object of type T and an error if any occurs during the fetching process.
// If error is nil, it indicates that the object was successfully fetched or built. So cache can be set.
// If the object is not found or cannot be built, it should return a zero value of type T and an error.
// Then the cache will not be set.
type FetchCached[T any] = func() (obj T, err error)

// GetEx retrieves an object from the cache using the provided key.
// And returns the object, a boolean indicating if it was found, and an error if any occurred.
// If the object is not found, it uses the builder function to create the object.
// When the builder returns an error, the cache will not be set. and found will be false.
func GetEx[T any](ctx context.Context, c Cache[T], key string, builder FetchCached[T], options ...Option) (T, bool, error) {
	return load[T](
		ctx,
		key,
		c.Get,
		func(ctx context.Context, k string, v T, opts ...Option) error {
			return Set[T](ctx, c, k, v, opts...)
		},
		builder,
		options...,
	)
}

// GetObjectEx retrieves an object from the cache using the provided key.
// Similar to GetEx, but for ByteCache with encoding/decoding support.
// If the object is not found in cache, it uses the builder function to create it and caches the result.
func GetObjectEx[T any, D codec.Decoder, E codec.Encoder](ctx context.Context, c ByteCache, d D, e E, key string, builder FetchCached[T], options ...Option) (T, bool, error) {
	return load[T](
		ctx,
		key,
		func(ctx context.Context, k string) (T, bool, error) {
			return GetObject[T, D](ctx, c, d, k)
		},
		func(ctx context.Context, k string, v T, opts ...Option) error {
			return SetObject[T, E](ctx, c, e, k, v, opts...)
		},
		builder,
		options...,
	)
}

// GetJsonEx retrieves a JSON object from the cache using the provided key.
// Similar to GetObjectEx, but specifically for JSON data with automatic encoding/decoding.
// If the object is not found in cache, it uses the builder function to create it and caches the result as JSON.
func GetJsonEx[T any](ctx context.Context, c ByteCache, key string, builder FetchCached[T], options ...Option) (T, bool, error) {
	return GetObjectEx[T, codec.DecoderFunc, codec.EncoderFunc](ctx, c, json.Unmarshal, json.Marshal, key, builder, options...)
}

func load[T any](
	ctx context.Context,
	key string,
	getter func(context.Context, string) (T, bool, error),
	setter func(context.Context, string, T, ...Option) error,
	builder FetchCached[T],
	options ...Option,
) (T, bool, error) {
	opts := newOptions(options...)
	obj, found, gErr := getter(ctx, key)
	if gErr != nil {
		var zero T
		return zero, false, gErr
	}
	if found {
		return obj, true, nil
	}
	if builder == nil {
		var zero T
		return zero, false, nil
	}
	build := func() (T, error) {
		nObj, err := builder()
		if err == nil {
			_ = setter(ctx, key, nObj, options...)
		}
		return nObj, err
	}
	if opts.singleflight != nil {
		originBuild := build
		build = func() (T, error) {
			val, err, _ := opts.singleflight.Do(key, func() (interface{}, error) {
				return originBuild()
			})
			return val.(T), err
		}
	}
	newObj, err := build()
	return newObj, err == nil, err
}
