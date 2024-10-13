package dao

import "sort"

func RemoveDuplicateAndZero(origin []int) []int {
	res := make([]int, len(origin))
	copy(res, origin)
	j := 0
	sort.Ints(res)
	for i := 0; i < len(res); i++ {
		if res[i] != 0 && (j == 0 || res[i] != res[j-1]) {
			res[j] = res[i]
			j++
		}
	}
	return res[:j]
}
