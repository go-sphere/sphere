// Package qiniuplain lets the in-process Kodo fake switch the qiniu driver
// from https to plain http, which is all the fake speaks. It is test plumbing
// kept out of the public qiniu API.
package qiniuplain

import (
	"sync"
	"sync/atomic"
)

var users atomic.Int64

// Acquire makes clients created from now on use plain http until the returned
// release is called. Calls nest: http stays enabled while any release is
// outstanding.
func Acquire() (release func()) {
	users.Add(1)
	return sync.OnceFunc(func() { users.Add(-1) })
}

// Enabled reports whether new qiniu clients should use plain http.
func Enabled() bool { return users.Load() > 0 }
