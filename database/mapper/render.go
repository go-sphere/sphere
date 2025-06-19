package mapper

import (
	"github.com/go-viper/mapstructure/v2"
	"golang.org/x/exp/constraints"
)

const DefaultPageSize = 20

func Map[S any, T any](source []S, mapper func(S) T) []T {
	result := make([]T, len(source))
	for i, s := range source {
		result[i] = mapper(s)
	}
	return result
}

func Group[S any, K comparable](source []S, keyFunc func(S) K) map[K]S {
	result := make(map[K]S, len(source))
	for _, s := range source {
		key := keyFunc(s)
		result[key] = s
	}
	return result
}

func MapStruct[S any, T any](source *S) *T {
	if source == nil {
		return nil
	}
	var target T
	err := mapstructure.WeakDecode(source, &target)
	if err != nil {
		return nil
	}
	return &target
}

func Page[P constraints.Integer](total, pageSize, defaultSize P) P {
	if total == 0 {
		return 0
	}
	if pageSize == 0 {
		pageSize = defaultSize
	}
	if pageSize == 0 {
		return total
	}
	page := total / pageSize
	if total%pageSize != 0 {
		page++
	}
	return page
}
