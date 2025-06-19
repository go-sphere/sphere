package mapper

import (
	"cmp"
	"slices"

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

type Pager[P constraints.Integer] func(total, pageSize P) P

func NewPager[P constraints.Integer](defaultSize P) Pager[P] {
	return func(total, pageSize P) P {
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
}

func Page[P constraints.Integer](total, pageSize, defaultSize P) P {
	return NewPager(defaultSize)(total, pageSize)
}

func UniqueSorted[T cmp.Ordered](origin []T) []T {
	var zero T
	seen := make(map[T]struct{})
	result := make([]T, 0, len(origin))
	for _, v := range origin {
		if v == zero {
			continue
		}
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	slices.Sort(result)
	return slices.Clone(result)
}
