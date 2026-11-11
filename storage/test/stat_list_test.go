package test

import (
	"bytes"
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/kvcache"
	"github.com/go-sphere/sphere/storage/local"
	"github.com/go-sphere/sphere/storage/storageerr"

	"github.com/go-sphere/sphere/cache/memory"
)

func newLocalStorage(t *testing.T) *local.Client {
	t.Helper()
	client, err := local.NewClient(local.Config{RootDir: t.TempDir()})
	if err != nil {
		t.Fatalf("local.NewClient() error = %v", err)
	}
	return client
}

func putFile(t *testing.T, ctx context.Context, store storage.FileUploader, key, body string) {
	t.Helper()
	if _, err := store.UploadFile(ctx, bytes.NewBufferString(body), key); err != nil {
		t.Fatalf("UploadFile(%q) error = %v", key, err)
	}
}

// statListFactories are the drivers that implement both FileStater and
// FileLister.
func statListFactories() []storageFactory {
	return []storageFactory{
		{name: "local", new: func(t *testing.T) storage.Storage { return newLocalStorage(t) }},
		{name: "s3", new: func(t *testing.T) storage.Storage { return newFakeS3Storage(t) }},
		{name: "qiniu", new: func(t *testing.T) storage.Storage { return newFakeQiniuStorage(t) }},
	}
}

func TestFileStaterContract(t *testing.T) {
	for _, factory := range statListFactories() {
		t.Run(factory.name, func(t *testing.T) {
			testFileStaterContract(t, factory.new(t))
		})
	}
}

func testFileStaterContract(t *testing.T, store storage.Storage) {
	ctx := context.Background()
	stater, ok := store.(storage.FileStater)
	if !ok {
		t.Fatalf("%T does not implement storage.FileStater", store)
	}

	const key = "docs/readme.txt"
	const body = "hello world"
	putFile(t, ctx, store, key, body)

	info, err := stater.StatFile(ctx, key)
	if err != nil {
		t.Fatalf("StatFile() error = %v", err)
	}
	if info.Size != int64(len(body)) {
		t.Fatalf("StatFile() size = %d, want %d", info.Size, len(body))
	}
	if !strings.HasPrefix(info.MIME, "text/plain") {
		t.Fatalf("StatFile() mime = %q, want text/plain prefix", info.MIME)
	}

	// StatFile must not read the body: the reported size must match a subsequent
	// download exactly.
	download, err := store.DownloadFile(ctx, key)
	if err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}
	_ = download.Reader.Close()
	if info.Size != download.Size {
		t.Fatalf("StatFile size %d != DownloadFile size %d", info.Size, download.Size)
	}

	if _, err = stater.StatFile(ctx, "missing/nope.txt"); !errors.Is(err, storageerr.ErrNotFound) {
		t.Fatalf("StatFile(missing) error = %v, want %v", err, storageerr.ErrNotFound)
	}
}

func TestFileListerContract(t *testing.T) {
	for _, factory := range statListFactories() {
		t.Run(factory.name, func(t *testing.T) {
			testFileListerContract(t, factory.new(t))
		})
	}
}

func testFileListerContract(t *testing.T, store storage.Storage) {
	ctx := context.Background()
	lister, ok := store.(storage.FileLister)
	if !ok {
		t.Fatalf("%T does not implement storage.FileLister", store)
	}

	// Keys are deliberately created out of lexical order to prove the listing
	// is sorted and paginates deterministically.
	for _, key := range []string{"list/c.txt", "list/a.txt", "list/b.txt", "other/d.txt"} {
		putFile(t, ctx, store, key, "x")
	}

	t.Run("prefix filters and sorts", func(t *testing.T) {
		keys, next, err := lister.ListFiles(ctx, "list/", "", 10)
		if err != nil {
			t.Fatalf("ListFiles() error = %v", err)
		}
		want := []string{"list/a.txt", "list/b.txt", "list/c.txt"}
		assertKeys(t, keys, want)
		if next != "" {
			t.Fatalf("next = %q, want empty (full page fit)", next)
		}
	})

	t.Run("cursor pagination", func(t *testing.T) {
		page1, next1, err := lister.ListFiles(ctx, "list/", "", 2)
		if err != nil {
			t.Fatalf("ListFiles() page1 error = %v", err)
		}
		assertKeys(t, page1, []string{"list/a.txt", "list/b.txt"})
		if next1 == "" {
			t.Fatal("next cursor is empty, want a resume cursor")
		}

		page2, next2, err := lister.ListFiles(ctx, "list/", next1, 2)
		if err != nil {
			t.Fatalf("ListFiles() page2 error = %v", err)
		}
		assertKeys(t, page2, []string{"list/c.txt"})
		if next2 != "" {
			t.Fatalf("next2 = %q, want empty (last page)", next2)
		}
	})

	t.Run("empty prefix lists everything", func(t *testing.T) {
		keys, _, err := lister.ListFiles(ctx, "", "", 100)
		if err != nil {
			t.Fatalf("ListFiles() error = %v", err)
		}
		assertKeys(t, keys, []string{"list/a.txt", "list/b.txt", "list/c.txt", "other/d.txt"})
	})

	t.Run("huge limit is an upper bound, not an allocation size", func(t *testing.T) {
		keys, next, err := lister.ListFiles(ctx, "list/", "", math.MaxInt)
		if err != nil {
			t.Fatalf("ListFiles() error = %v", err)
		}
		assertKeys(t, keys, []string{"list/a.txt", "list/b.txt", "list/c.txt"})
		if next != "" {
			t.Fatalf("next = %q, want empty", next)
		}
	})
}

// TestFileListerSkipsUnaddressableKeys runs the listers whose backend can hold
// keys that storage.NormalizeKey rejects or rewrites (written by other tools,
// such as an S3 console folder marker). Listing them would hand out keys that
// every other method resolves to a different object, so they are skipped,
// whatever page boundary they fall on.
func TestFileListerSkipsUnaddressableKeys(t *testing.T) {
	tests := []struct {
		name string
		new  func(t *testing.T) (storage.FileLister, func(key string))
	}{
		{name: "s3", new: func(t *testing.T) (storage.FileLister, func(key string)) {
			client, fake := newFakeS3StorageWithServer(t)
			return client, func(key string) { fake.Put(key, []byte("x"), "") }
		}},
		{name: "qiniu", new: func(t *testing.T) (storage.FileLister, func(key string)) {
			client, fake := newFakeQiniuStorageWithServer(t)
			return client, func(key string) { fake.Put(key, []byte("x"), "") }
		}},
	}
	addressable := []string{"raw/a.txt", "raw/e.txt", "raw/h.txt"}
	unaddressable := []string{"/lead.txt", "raw/b/", "raw/c//d.txt", "raw/d/./x.txt", "raw/f/../g.txt", "raw/g/"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			lister, seed := tt.new(t)
			for _, key := range append(append([]string{}, addressable...), unaddressable...) {
				seed(key)
			}
			for _, limit := range []int{1, 2, 3, 100} {
				var got []string
				cursor := ""
				for pages := 0; ; pages++ {
					if pages > 20 {
						t.Fatalf("limit %d: listing did not terminate", limit)
					}
					keys, next, err := lister.ListFiles(ctx, "", cursor, limit)
					if err != nil {
						t.Fatalf("limit %d: ListFiles() error = %v", limit, err)
					}
					if len(keys) > limit {
						t.Fatalf("limit %d: page has %d keys", limit, len(keys))
					}
					got = append(got, keys...)
					if next == "" {
						break
					}
					cursor = next
				}
				if strings.Join(got, ",") != strings.Join(addressable, ",") {
					t.Fatalf("limit %d: listed %q, want %q", limit, got, addressable)
				}
			}
		})
	}
}

// TestOptionalCapabilitiesAreProbed documents that the optional capabilities are
// detected by type assertion and are absent on backends that cannot serve them
// cheaply (kvcache here).
func TestOptionalCapabilitiesAreProbed(t *testing.T) {
	storeCache := memory.NewByteCache()
	t.Cleanup(func() { _ = storeCache.Close() })
	kv, err := kvcache.NewClient(kvcache.Config{}, storeCache)
	if err != nil {
		t.Fatalf("kvcache.NewClient() error = %v", err)
	}

	if _, ok := any(kv).(storage.FileStater); ok {
		t.Fatal("kvcache.Client unexpectedly implements storage.FileStater")
	}
	if _, ok := any(kv).(storage.FileLister); ok {
		t.Fatal("kvcache.Client unexpectedly implements storage.FileLister")
	}
}

func assertKeys(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keys = %v, want %v", got, want)
		}
	}
}
