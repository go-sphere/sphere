package mapper

import (
	"cmp"
	"slices"

	"github.com/go-viper/mapstructure/v2"
	"golang.org/x/exp/constraints"
)

// DefaultPageSize defines the standard page size used for pagination operations.
const DefaultPageSize = 20

// Map transforms a slice of source items to a slice of target items using the provided entmapper function.
// It applies the entmapper function to each element and returns a new slice with the transformed results.
func Map[S any, T any](source []S, mapper func(S) T) []T {
	result := make([]T, len(source))
	for i, s := range source {
		result[i] = mapper(s)
	}
	return result
}

// Group creates a map from a slice by extracting keys using the provided keyFunc.
// If multiple items have the same key, the last item encountered will be kept.
// This is useful for creating lookup tables from slices.
func Group[S any, K comparable](source []S, keyFunc func(S) K) map[K]S {
	result := make(map[K]S, len(source))
	for _, s := range source {
		key := keyFunc(s)
		result[key] = s
	}
	return result
}

// MapStruct converts between struct types using mapstructure with weak decoding.
// It handles type conversions automatically and returns nil if the source is nil
// or if the conversion fails. This is useful for converting between similar struct types.
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

// Page calculates pagination values based on total items and page size.
// It returns the number of pages needed and the effective page size to use.
// If pageSize is invalid, it uses the defaultSize parameter.
func Page[P constraints.Integer](total, pageSize, defaultSize P) (page P, size P) {
	if pageSize <= 0 {
		pageSize = max(1, defaultSize)
	}
	if total == 0 {
		return 0, pageSize
	}
	page = total / pageSize
	if total%pageSize != 0 {
		page++
	}
	return page, pageSize
}

// UniqueSorted removes duplicates from a slice and returns a sorted copy.
// Zero values are excluded from the result. The original slice is not modified.
// This function is useful for creating clean, deduplicated lists.
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
