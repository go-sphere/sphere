package cache

import (
	"context"
	"encoding/json"
	"golang.org/x/sync/singleflight"
	"time"
)

type Encoder interface {
	Marshal(val any) ([]byte, error)
}

type EncoderFunc func(val any) ([]byte, error)

func (e EncoderFunc) Marshal(val any) ([]byte, error) {
	return e(val)
}

type Decoder interface {
	Unmarshal(data []byte, val any) error
}

type DecoderFunc func(data []byte, val any) error

func (d DecoderFunc) Unmarshal(data []byte, val any) error {
	return d(data, val)
}

func Load[T any, D Decoder](ctx context.Context, c ByteCache, d D, key string) (*T, error) {
	data, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var value T
	err = d.Unmarshal(data, &value)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func Save[T any, E Encoder](ctx context.Context, c ByteCache, e E, key string, value *T, expiration time.Duration) error {
	data, err := e.Marshal(value)
	if err != nil {
		return err
	}
	return c.Set(ctx, key, data, expiration)
}

func LoadEx[T any, D Decoder, E Encoder](ctx context.Context, c ByteCache, d D, e E, sf *singleflight.Group, key string, expiration time.Duration, builder func() (obj *T, err error)) (*T, error) {
	obj, err := Load[T, D](ctx, c, d, key)
	if err == nil {
		return obj, nil
	}
	build, err, _ := sf.Do(key, func() (interface{}, error) {
		nObj, nErr := builder()
		if nErr != nil {
			return nil, nErr
		}
		nErr = Save[T, E](ctx, c, e, key, nObj, expiration)
		if nErr != nil {
			return nil, nErr
		}
		return nObj, nil
	})
	if err != nil {
		return nil, err
	}
	return build.(*T), nil
}

func LoadJson[T any](ctx context.Context, c ByteCache, key string) (*T, error) {
	return Load[T, DecoderFunc](ctx, c, json.Unmarshal, key)
}

func SaveJson[T any](ctx context.Context, c ByteCache, key string, value *T, expiration time.Duration) error {
	return Save[T, EncoderFunc](ctx, c, json.Marshal, key, value, expiration)
}

func LoadJsonEx[T any](ctx context.Context, c ByteCache, sf *singleflight.Group, key string, expiration time.Duration, builder func() (obj *T, err error)) (*T, error) {
	return LoadEx[T, DecoderFunc, EncoderFunc](ctx, c, json.Unmarshal, json.Marshal, sf, key, expiration, builder)
}
