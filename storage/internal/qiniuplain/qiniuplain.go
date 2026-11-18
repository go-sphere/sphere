// Package qiniuplain lets the in-process Kodo fake switch the qiniu driver
// from https to plain http, which is all the fake speaks. It is test plumbing
// kept out of the public qiniu API.
package qiniuplain

import "sync"

var (
	mu    sync.Mutex
	users = map[string]int{}
)

// Acquire makes clients created for accessKey use plain http until the returned
// release is called. Calls nest: http stays enabled while any release is
// outstanding.
func Acquire(accessKey string) (release func()) {
	if accessKey == "" {
		return func() {}
	}
	mu.Lock()
	users[accessKey]++
	mu.Unlock()
	return sync.OnceFunc(func() {
		mu.Lock()
		if users[accessKey] <= 1 {
			delete(users, accessKey)
		} else {
			users[accessKey]--
		}
		mu.Unlock()
	})
}

// Enabled reports whether new qiniu clients for accessKey should use plain http.
func Enabled(accessKey string) bool {
	if accessKey == "" {
		return false
	}
	mu.Lock()
	defer mu.Unlock()
	return users[accessKey] > 0
}
