package cache

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"golang.org/x/sync/singleflight"
)

const neverExpire = time.Duration(-1)

func IsZero[T any](t T) bool {
	var zero T
	return reflect.DeepEqual(t, zero)
}

type Options struct {
	expiration   time.Duration
	singleflight *singleflight.Group
}

func newOptions(options ...Option) *Options {
	opt := &Options{
		expiration:   neverExpire,
		singleflight: nil,
	}
	for _, option := range options {
		option(opt)
	}
	return opt
}

type Option func(o *Options)

func WithExpiration(expiration time.Duration) Option {
	return func(o *Options) {
		o.expiration = expiration
	}
}

func WithNeverExpire() Option {
	return func(o *Options) {
		o.expiration = neverExpire
	}
}

func WithSingleflight(single *singleflight.Group) Option {
	return func(o *Options) {
		o.singleflight = single
	}
}

func Set[T any](ctx context.Context, c Cache[T], key string, value T, options ...Option) error {
	opts := newOptions(options...)
	if opts.expiration == neverExpire {
		return c.Set(ctx, key, value)
	} else {
		return c.SetWithTTL(ctx, key, value, opts.expiration)
	}
}

func SetObject[T any, E Encoder](ctx context.Context, c ByteCache, e E, key string, value T, options ...Option) error {
	data, err := e.Marshal(value)
	if err != nil {
		return err
	}
	return Set(ctx, c, key, data, options...)
}

func SetJson[T any](ctx context.Context, c ByteCache, key string, value T, options ...Option) error {
	return SetObject[T, EncoderFunc](ctx, c, json.Marshal, key, value, options...)
}

func GetObject[T any, D Decoder](ctx context.Context, c ByteCache, d D, key string) (T, bool, error) {
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

func GetJson[T any](ctx context.Context, c ByteCache, key string, options ...Option) (T, bool, error) {
	return GetObject[T, DecoderFunc](ctx, c, json.Unmarshal, key)
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
// Something like GetEx, but for ByteCache.
func GetObjectEx[T any, D Decoder, E Encoder](ctx context.Context, c ByteCache, d D, e E, key string, builder FetchCached[T], options ...Option) (T, bool, error) {
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
// Similar to GetObjectEx, but specifically for JSON data.
func GetJsonEx[T any](ctx context.Context, c ByteCache, key string, builder FetchCached[T], options ...Option) (T, bool, error) {
	return GetObjectEx[T, DecoderFunc, EncoderFunc](ctx, c, json.Unmarshal, json.Marshal, key, builder, options...)
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
