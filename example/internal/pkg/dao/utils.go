package dao

import (
	"cmp"
	"slices"
)

func RemoveDuplicateAndZero[T cmp.Ordered](origin []T) []T {
	if len(origin) == 0 {
		return []T{}
	}

	res := make([]T, len(origin))
	copy(res, origin)

	slices.Sort(res)

	var zero T
	write := 0
	for read := 0; read < len(res); read++ {
		if res[read] != zero && (write == 0 || res[read] != res[write-1]) {
			res[write] = res[read]
			write++
		}
	}

	return res[:write]
}
