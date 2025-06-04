package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"
)

// ErrNotFound is returned when a cache entry is not found, Only used for Load and Get methods.
var ErrNotFound = fmt.Errorf("not found")

// NeverExpire is a special value for expiration that indicates the cache entry should never expire.
const NeverExpire = time.Duration(-1)

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

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

// Save stores a value in the cache with the given key and expiration.
// If expiration is NeverExpire, the value will not expire.
func Save[T any, E Encoder](ctx context.Context, c ByteCache, e E, key string, value *T, expiration time.Duration) error {
	data, err := e.Marshal(value)
	if err != nil {
		return err
	}
	if expiration == NeverExpire {
		return c.Set(ctx, key, data)
	} else {
		return c.SetWithTTL(ctx, key, data, expiration)
	}
}

// SaveJson stores a value in the cache as JSON with the given key and expiration.
// If expiration is NeverExpire, the value will not expire.
func SaveJson[T any](ctx context.Context, c ByteCache, key string, value *T, expiration time.Duration) error {
	return Save[T, EncoderFunc](ctx, c, json.Marshal, key, value, expiration)
}

// Load retrieves a value from the cache by key and decodes it using the provided decoder.
func Load[T any, D Decoder](ctx context.Context, c ByteCache, d D, key string) (*T, error) {
	data, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, ErrNotFound
	}
	var value T
	err = d.Unmarshal(*data, &value)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// LoadEx retrieves a value from the cache by key and decodes it using the provided decoder.
// If the value does not exist, it calls the builder function to create the value,
// stores it in the cache, and returns it.
// It uses singleflight to prevent duplicate builds for concurrent requests.
// Notes:
// - If builder returns nil, nothing will be stored in the cache
// - If expiration is NeverExpire, the value will persist indefinitely
// - Never returns ErrNotFound - will always attempt to build if not found
// Returns:
// - The cached/built value (or nil)
// - Any error that occurred during retrieval or building (NotFound is not returned, but other errors are)
func LoadEx[T any, D Decoder, E Encoder](ctx context.Context, c ByteCache, d D, e E, sf *singleflight.Group, key string, expiration time.Duration, builder func() (obj *T, err error)) (*T, error) {
	obj, err := Load[T, D](ctx, c, d, key)
	if err == nil {
		return obj, nil
	}
	if !IsNotFound(err) {
		return nil, err // If it's not a NotFound error, return it directly
	}
	build, err, _ := sf.Do(key, func() (interface{}, error) {
		nObj, nErr := builder()
		if nErr != nil {
			return nil, nErr
		}
		if nObj == nil {
			return nil, nil
		}
		nErr = Save[T, E](ctx, c, e, key, nObj, expiration)
		if nErr != nil {
			return nObj, nErr
		}
		return nObj, nil
	})
	if err != nil {
		return nil, err
	}
	if build == nil {
		return nil, nil
	}
	return build.(*T), nil
}

// LoadJson is a convenience function for Load that uses JSON decoding.
func LoadJson[T any](ctx context.Context, c ByteCache, key string) (*T, error) {
	return Load[T, DecoderFunc](ctx, c, json.Unmarshal, key)
}

// LoadJsonEx is a convenience function for LoadEx that uses JSON encoding and decoding.
func LoadJsonEx[T any](ctx context.Context, c ByteCache, sf *singleflight.Group, key string, expiration time.Duration, builder func() (obj *T, err error)) (*T, error) {
	return LoadEx[T, DecoderFunc, EncoderFunc](ctx, c, json.Unmarshal, json.Marshal, sf, key, expiration, builder)
}

// Set stores a value in the cache with the given key and expiration. If expiration is NeverExpire, the value will not expire.
func Set[T any](ctx context.Context, c Cache[T], key string, value *T, expiration time.Duration) error {
	if value == nil {
		return c.Del(ctx, key)
	} else if expiration == NeverExpire {
		return c.Set(ctx, key, *value)
	} else {
		return c.SetWithTTL(ctx, key, *value, expiration)
	}
}

// Get retrieves a value from the cache by key. If the key does not exist, it returns ErrNotFound.
func Get[T any](ctx context.Context, c Cache[T], key string) (*T, error) {
	data, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, ErrNotFound
	}
	return data, nil
}

// GetEx retrieves a value from the cache. If the value doesn't exist,
// it calls the builder function to create the value, stores it in the cache, and returns it.
//
// Notes:
// - If builder returns nil, nothing will be stored in the cache
// - If expiration is NeverExpire, the value will persist indefinitely
// - Never returns ErrNotFound - will always attempt to build if not found
// - Uses singleflight to prevent duplicate builds for concurrent requests
//
// Returns:
// - The cached/built value (or nil)
// - Any error that occurred during retrieval or building (NotFound is not returned, but other errors are)
func GetEx[T any](ctx context.Context, c Cache[T], sf *singleflight.Group, key string, expiration time.Duration, builder func() (obj *T, err error)) (*T, error) {
	obj, err := Get[T](ctx, c, key)
	if err == nil {
		return obj, nil
	}
	if !IsNotFound(err) {
		return nil, err // If it's not a NotFound error, return it directly
	}
	build, err, _ := sf.Do(key, func() (interface{}, error) {
		nObj, nErr := builder()
		if nErr != nil {
			return nil, nErr
		}
		if nObj == nil {
			return nil, nil
		}
		nErr = Set[T](ctx, c, key, nObj, expiration)
		if nErr != nil {
			return nObj, nErr // Save the object error, but build succeeded, return it
		}
		return nObj, nil
	})
	if err != nil {
		return nil, err
	}
	if build == nil {
		return nil, nil
	}
	return build.(*T), nil
}
