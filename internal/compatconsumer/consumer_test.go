// Package compatconsumer is a compile-time check that stable constructors
// used by generated service templates still type-check against the public
// interfaces. It is not a runtime test suite.
package compatconsumer

import (
	"testing"

	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/cache/mcache"
	"github.com/go-sphere/sphere/core/boot"
	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/mq"
	memorymq "github.com/go-sphere/sphere/mq/memory"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/local"
)

var (
	_ cache.ByteCache      = (*mcache.Map[string, []byte])(nil)
	_ mq.MessageQueue[int] = (*memorymq.MessageQueue[int])(nil)
	_ storage.Storage      = (*local.Client)(nil)
	_ task.Task            = (*boot.Application)(nil)
	_ httpz.DataResponse[int]
	_ httpz.ErrorResponse
)

func TestStableConstructorsCompile(t *testing.T) {
	byteCache := mcache.NewByteCache()
	t.Cleanup(func() { _ = byteCache.Close() })

	typedCache := cache.NewJsonCache[map[string]int](byteCache)
	if typedCache == nil {
		t.Fatal("NewJsonCache returned nil")
	}

	messageQueue := memorymq.NewMessageQueue[int]()
	t.Cleanup(func() { _ = messageQueue.Close() })

	if app := boot.NewApplication(); app == nil {
		t.Fatal("NewApplication returned nil")
	}

	store, err := local.NewClient(local.Config{RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if store == nil {
		t.Fatal("NewClient returned nil storage")
	}
}
