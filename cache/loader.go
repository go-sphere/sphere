package cache

import (
	"context"
	"encoding/json"
	"errors"
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

func GetEx[T any](ctx context.Context, c Cache[T], key string, builder func() (obj *T, err error), options ...Option) (T, bool, error) {
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

func GetObjectEx[T any, D Decoder, E Encoder](ctx context.Context, c ByteCache, d D, e E, key string, builder func() (*T, error), options ...Option) (T, bool, error) {
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

func GetJsonEx[T any](ctx context.Context, c ByteCache, key string, builder func() (obj *T, err error), options ...Option) (T, bool, error) {
	return GetObjectEx[T, DecoderFunc, EncoderFunc](ctx, c, json.Unmarshal, json.Marshal, key, builder, options...)
}

func load[T any](
	ctx context.Context,
	key string,
	getter func(context.Context, string) (T, bool, error),
	setter func(context.Context, string, T, ...Option) error,
	builder func() (*T, error),
	options ...Option,
) (T, bool, error) {
	opts := newOptions(options...)
	obj, found, cErr := getter(ctx, key)
	if cErr != nil {
		var zero T
		return zero, false, cErr
	}
	if found {
		return obj, true, nil
	}
	if builder == nil {
		var zero T
		return zero, false, nil
	}
	build := func() (*T, error) {
		nObj, err := builder()
		if err != nil {
			return nil, err
		}
		if nObj == nil {
			return nil, nil
		}
		return nObj, setter(ctx, key, *nObj, options...)
	}
	if opts.singleflight != nil {
		originBuild := build
		build = func() (*T, error) {
			val, err, _ := opts.singleflight.Do(key, func() (interface{}, error) {
				return originBuild()
			})
			if val == nil {
				return nil, err
			}
			nObj, ok := val.(*T)
			if !ok {
				return nil, errors.New("cast value failed")
			}
			return nObj, err
		}
	}
	newObj, err := build()
	if newObj != nil {
		obj = *newObj
		return obj, true, err
	} else {
		var zero T
		return zero, false, err
	}
}
