// Package redistest starts a miniredis server and returns a go-redis client
// for tests.
//
// It is not a real Redis: commands miniredis does not implement fail, and
// TTLs advance through miniredis time rather than wall-clock. A background
// ticker fast-forwards miniredis 5ms every 5ms so Redis TTLs fire without
// waiting. t.Cleanup stops the ticker and closes the client.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/test/redistest"
//
//	func TestSomething(t *testing.T) {
//		client := redistest.NewTestRedisClient(t)
//		if err := client.Set(t.Context(), "k", "v", 0).Err(); err != nil {
//			t.Fatal(err)
//		}
//	}
package redistest
