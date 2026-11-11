package meilisearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTaskServer stubs the Meilisearch endpoints Index and Delete use. Each
// task reports "enqueued" on its first status check and "succeeded" after, as
// a real server does for any task that is not instantly processed.
func newTaskServer(t *testing.T) *httptest.Server {
	t.Helper()
	var polls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/tasks/") {
			if polls.Add(1)%2 == 1 {
				_, _ = w.Write([]byte(`{"uid":1,"status":"enqueued"}`))
				return
			}
			_, _ = w.Write([]byte(`{"uid":1,"status":"succeeded"}`))
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"taskUid":1,"status":"enqueued"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestSearcherConcurrentIndex pins that one Searcher is safe for concurrent
// Index calls. meilisearch-go records the primary key on its index handle on
// every add, so a handle shared across calls is a data race (run with -race).
func TestSearcherConcurrentIndex(t *testing.T) {
	manager, err := NewServiceManager(Config{Host: newTaskServer(t).URL})
	if err != nil {
		t.Fatal(err)
	}
	searcher, err := NewSearcher[Article](manager, "articles", PrimaryKey("id"))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			if err := searcher.Index(context.Background(), Article{ID: int64(i)}); err != nil {
				t.Errorf("index: %v", err)
			}
		})
	}
	wg.Wait()
}

// TestIndexDoesNotWaitAWholeSecondPerTask pins the task poll interval: a task
// still enqueued on the first check must not cost a full second per call.
func TestIndexDoesNotWaitAWholeSecondPerTask(t *testing.T) {
	manager, err := NewServiceManager(Config{Host: newTaskServer(t).URL})
	if err != nil {
		t.Fatal(err)
	}
	searcher, err := NewSearcher[Article](manager, "articles", PrimaryKey("id"))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := searcher.Index(context.Background(), Article{ID: 1}); err != nil {
		t.Fatalf("index: %v", err)
	}
	if err := searcher.Delete(context.Background(), "1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 500*time.Millisecond {
		t.Fatalf("Index+Delete took %v, want well under a second", elapsed)
	}
}
