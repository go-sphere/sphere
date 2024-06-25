package cache

import (
	"encoding/json"
	"golang.org/x/sync/singleflight"
)

type Cache interface {
	Set(key string, value []byte, expireSeconds int) error
	Delete(key string) error
	Get(key string) ([]byte, error)
	Reset() error
}

func NewCache() Cache {
	return NewMemoryCache()
}

func LoadJson[T any](c Cache, key string) (*T, error) {
	data, err := c.Get(key)
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

func SaveJson[T any](c Cache, key string, value *T, expireSeconds int) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Set(key, data, expireSeconds)
}

func LoadJsonEx[T any](c Cache, sf *singleflight.Group, key string, expireSeconds int, builder func() (obj *T, err error)) (*T, error) {
	obj, err := LoadJson[T](c, key)
	if err == nil {
		return obj, nil
	}
	build, err, _ := sf.Do(key, func() (interface{}, error) {
		nObj, nErr := builder()
		if nErr != nil {
			return nil, nErr
		}
		nErr = SaveJson[T](c, key, nObj, expireSeconds)
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
