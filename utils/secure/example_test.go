package secure_test

import (
	"fmt"

	"github.com/go-sphere/sphere/utils/secure"
)

func ExampleCryptPassword() {
	hash, err := secure.CryptPassword("s3cret-password")
	if err != nil {
		fmt.Println(err)
		return
	}
	// Each call uses a fresh salt, so store the hash and compare with
	// IsPasswordMatch instead of re-hashing.
	fmt.Println(secure.IsPasswordMatch("s3cret-password", hash))
	fmt.Println(secure.IsPasswordMatch("wrong", hash))
	// Output:
	// true
	// false
}

func ExampleCensorString() {
	fmt.Println(secure.CensorString("13812345678", 11))
	fmt.Println(secure.CensorString("alice@example.com", 6))
	fmt.Println(secure.CensorString("密码", 4))
	// Output:
	// 1*********8
	// a****m
	// 密**码
}

func ExampleRandString() {
	token := secure.RandString(32)
	fmt.Println(len(token))
	fmt.Println(secure.RandString(0) == "")
	// Output:
	// 32
	// true
}
