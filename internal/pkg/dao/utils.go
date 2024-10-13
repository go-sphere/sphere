package dao

type Integer interface {
	int | int64
}

func RemoveDuplicateAndZero[T Integer](origin []T) []T {
	res := make([]T, len(origin))
	copy(res, origin)
	j := 0
	for i := 0; i < len(res); i++ {
		if res[i] != 0 && (j == 0 || res[i] != res[j-1]) {
			res[j] = res[i]
			j++
		}
	}
	return res[:j]
}
