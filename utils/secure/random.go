package secure

import "math/rand/v2"

func RandString(length int) string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	res := make([]byte, length)
	for i := range res {
		res[i] = chars[rand.IntN(len(chars))]
	}
	return string(res)
}
