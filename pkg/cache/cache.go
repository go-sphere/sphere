package cache

import (
	"context"
	"encoding/json"
	"golang.org/x/sync/singleflight"
	"time"
)

type Encoder interface {
	Encode(val any) ([]byte, error)
}

type Cache[S any] interface {
	Set(ctx context.Context, key string, val S, expiration time.Duration) error
	Get(ctx context.Context, key string) (S, error)
	MultiSet(ctx context.Context, valMap map[string]S, expiration time.Duration) error
	MultiGet(ctx context.Context, keys []string) (map[string]S, error)
	Del(ctx context.Context, keys ...string) error
	DelAll(ctx context.Context) error
}

type ByteCache Cache[[]byte]

func NewCache() ByteCache {
	return NewMemoryCache()
}

func LoadJson[T any](ctx context.Context, c ByteCache, key string) (*T, error) {
	data, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var value T
	err = json.Unmarshal(data, &value)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func SaveJson[T any](ctx context.Context, c ByteCache, key string, value *T, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Set(ctx, key, data, expiration)
}

func LoadJsonEx[T any](ctx context.Context, c ByteCache, sf *singleflight.Group, key string, expiration time.Duration, builder func() (obj *T, err error)) (*T, error) {
	obj, err := LoadJson[T](ctx, c, key)
	if err == nil {
		return obj, nil
	}
	build, err, _ := sf.Do(key, func() (interface{}, error) {
		nObj, nErr := builder()
		if nErr != nil {
			return nil, nErr
		}
		nErr = SaveJson[T](ctx, c, key, nObj, expiration)
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
