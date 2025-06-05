package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"
)

// ErrNotFound is returned when a cache entry is not found, typically when a key does not exist in the cache.
// For use by other packages that implement the Cache interface
var ErrNotFound = fmt.Errorf("not found")

// NeverExpire is a special value for expiration that indicates the cache entry should never expire.
// Only used when calling functions such as Get, Set, GetObject, SetObject, etc., of the current package
const NeverExpire = time.Duration(-1)

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// Set stores a value in the cache with the given key and expiration.
// If expiration is NeverExpire, the value will not expire.
// If value is nil, it deletes the key from the cache.
func Set[T any](ctx context.Context, c Cache[T], key string, value *T, expiration time.Duration) error {
	if value == nil {
		return c.Del(ctx, key)
	} else if expiration == NeverExpire {
		return c.Set(ctx, key, *value)
	} else {
		return c.SetWithTTL(ctx, key, *value, expiration)
	}
}

// SetObject stores a value in the cache with the given key and expiration.
// If expiration is NeverExpire, the value will not expire.
// If value is nil, it deletes the key from the cache.
func SetObject[T any, E Encoder](ctx context.Context, c ByteCache, e E, key string, value *T, expiration time.Duration) error {
	if value == nil {
		return c.Del(ctx, key)
	}
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

// SetJson stores a value in the cache as JSON with the given key and expiration.
// If expiration is NeverExpire, the value will not expire.
// If value is nil, it deletes the key from the cache.
func SetJson[T any](ctx context.Context, c ByteCache, key string, value *T, expiration time.Duration) error {
	return SetObject[T, EncoderFunc](ctx, c, json.Marshal, key, value, expiration)
}

// Get retrieves a value from the cache by key. If the key does not exist, it returns (nil, nil).
func Get[T any](ctx context.Context, c Cache[T], key string) (*T, error) {
	data, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	return data, nil
}

// GetX retrieves a value from the cache by key and returns it as a type T. Ignoring errors. Only returns (zero, false) if the key does not exist.
func GetX[T any](ctx context.Context, c Cache[T], key string) (T, bool) {
	var zero T
	data, err := c.Get(ctx, key)
	if err != nil {
		return zero, false
	}
	if data == nil {
		return zero, false
	}
	return *data, true
}

// GetObject retrieves a value from the cache by key and decodes it using the provided decoder. If the value does not exist, it returns (nil, nil).
func GetObject[T any, D Decoder](ctx context.Context, c ByteCache, d D, key string) (*T, error) {
	data, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	var value T
	err = d.Unmarshal(*data, &value)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// GetObjectEx retrieves a value from the cache by key and decodes it using the provided decoder.
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
func GetObjectEx[T any, D Decoder, E Encoder](ctx context.Context, c ByteCache, d D, e E, sf *singleflight.Group, key string, expiration time.Duration, builder func() (obj *T, err error)) (*T, error) {
	obj, err := GetObject[T, D](ctx, c, d, key)
	if err == nil && obj != nil {
		return obj, nil // If the object is found in the cache, return it directly
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
		nErr = SetObject[T, E](ctx, c, e, key, nObj, expiration)
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

// GetJson is a convenience function for GetObject that uses JSON decoding.
func GetJson[T any](ctx context.Context, c ByteCache, key string) (*T, error) {
	return GetObject[T, DecoderFunc](ctx, c, json.Unmarshal, key)
}

// GetJsonEx is a convenience function for GetObjectEx that uses JSON encoding and decoding.
func GetJsonEx[T any](ctx context.Context, c ByteCache, sf *singleflight.Group, key string, expiration time.Duration, builder func() (obj *T, err error)) (*T, error) {
	return GetObjectEx[T, DecoderFunc, EncoderFunc](ctx, c, json.Unmarshal, json.Marshal, sf, key, expiration, builder)
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
	if err == nil && obj != nil {
		return obj, nil // If the object is found in the cache, return it directly
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
			return nObj, nErr // SetObject the object error, but build succeeded, return it
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
