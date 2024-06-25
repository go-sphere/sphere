package dao

import (
	"math/rand"
	"sort"
	"testing"
)

func TestRemoveDuplicateAndZero(t *testing.T) {

	duplicateAndZero := func(id []int) []int {
		set := make(map[int]struct{})
		for _, v := range id {
			if v != 0 {
				set[v] = struct{}{}
			}
		}
		result := make([]int, 0, len(set))
		for k := range set {
			result = append(result, k)
		}
		sort.Ints(result)
		return result
	}

	genRandomArray := func(len int) []int {
		res := make([]int, len)
		for i := 0; i < len; i++ {
			res[i] = rand.Intn(100)
			if rand.Intn(5) == 0 {
				res[i] = 0
			}
		}
		return res
	}

	var tests [][]int

	for i := 0; i < 1000; i++ {
		origin := genRandomArray(i * 10)
		tests = append(tests, origin)
	}

	equal := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i, v := range a {
			if v != b[i] {
				return false
			}
		}
		return true

	}
	for _, tt := range tests {
		result := duplicateAndZero(tt)
		if !equal(RemoveDuplicateAndZero(tt), result) {
			t.Errorf("RemoveDuplicateAndZero(%v) = %v, want %v", tt, RemoveDuplicateAndZero(tt), result)
		}
	}
}
