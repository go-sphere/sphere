package meilisearch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-sphere/sphere/search"
)

type testDoc struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

func TestPrimaryKeyAndServiceManagerClose(t *testing.T) {
	if got := PrimaryKey(""); got != nil {
		t.Fatalf("PrimaryKey(\"\") = %v, want nil", *got)
	}
	got := PrimaryKey("id")
	if got == nil || *got != "id" {
		t.Fatalf("PrimaryKey(\"id\") = %v, want \"id\"", got)
	}

	var nilMgr *ServiceManager
	if err := nilMgr.Close(); err != nil {
		t.Fatalf("(*ServiceManager)(nil).Close() = %v, want nil", err)
	}

	mgr, err := NewServiceManager(Config{Host: "http://127.0.0.1:0", APIKey: "k"})
	if err != nil {
		t.Fatalf("NewServiceManager() error = %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("mgr.Close() error = %v", err)
	}
}

func TestSearcherEmptyInputsMakeZeroHTTPRequests(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	mgr, err := NewServiceManager(Config{Host: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("NewServiceManager() error = %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	s, err := NewSearcher[testDoc](mgr, "docs", PrimaryKey("id"))
	if err != nil {
		t.Fatalf("NewSearcher() error = %v", err)
	}

	if err := s.Index(context.Background()); err != nil {
		t.Fatalf("empty Index(bg) = %v, want nil", err)
	}
	if err := s.Delete(context.Background()); err != nil {
		t.Fatalf("empty Delete(bg) = %v, want nil", err)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Index(canceledCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("empty Index(canceled) = %v, want context.Canceled", err)
	}
	if err := s.Delete(canceledCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("empty Delete(canceled) = %v, want context.Canceled", err)
	}

	if got := calls.Load(); got != 0 {
		t.Fatalf("HTTP call count = %d, want 0", got)
	}
}

func TestSearcherHTTPIndexDeleteSearch(t *testing.T) {
	var (
		mu             sync.Mutex
		taskPollCount  int
		taskStatus     = "succeeded"
		taskErrPayload map[string]string
		lastAuth       string
		lastAddQuery   string
		lastAddBody    []testDoc
		lastDeleteIDs  []string
		lastSearchReq  map[string]any
		searchRespBody string
		forceStatus    int
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		lastAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		if forceStatus != 0 {
			w.WriteHeader(forceStatus)
			_, _ = w.Write([]byte(`{"message":"boom","code":"internal","type":"internal","link":""}`))
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/indexes/docs/documents":
			lastAddQuery = r.URL.RawQuery
			if err := json.Unmarshal(body, &lastAddBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			taskPollCount = 0
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":42,"indexUid":"docs","status":"enqueued","type":"documentAdditionOrUpdate"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/indexes/docs/documents/delete-batch":
			if err := json.Unmarshal(body, &lastDeleteIDs); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			taskPollCount = 0
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":43,"indexUid":"docs","status":"enqueued","type":"documentDeletion"}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/tasks/"):
			taskPollCount++
			uid := strings.TrimPrefix(r.URL.Path, "/tasks/")
			status := taskStatus
			if taskPollCount == 1 && status == "succeeded" {
				status = "processing"
			}
			taskUID := int64(42)
			if uid == "43" {
				taskUID = 43
			}
			resp := map[string]any{
				"uid":      taskUID,
				"taskUid":  taskUID,
				"indexUid": "docs",
				"status":   status,
			}
			if taskErrPayload != nil && status != "processing" {
				resp["error"] = taskErrPayload
			}
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodPost && r.URL.Path == "/indexes/docs/search":
			if err := json.Unmarshal(body, &lastSearchReq); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(searchRespBody))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	mgr, err := NewServiceManager(Config{Host: srv.URL, APIKey: "secret-key"})
	if err != nil {
		t.Fatalf("NewServiceManager() error = %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	s, err := NewSearcher[testDoc](mgr, "docs", PrimaryKey("id"))
	if err != nil {
		t.Fatalf("NewSearcher() error = %v", err)
	}

	ctx := context.Background()

	t.Run("index waits for task completion", func(t *testing.T) {
		if err := s.Index(ctx, testDoc{ID: 1, Title: "alpha"}); err != nil {
			t.Fatalf("Index() error = %v", err)
		}
		mu.Lock()
		defer mu.Unlock()
		if lastAuth != "Bearer secret-key" {
			t.Fatalf("Authorization = %q, want Bearer secret-key", lastAuth)
		}
		if lastAddQuery != "primaryKey=id" {
			t.Fatalf("query = %q, want primaryKey=id", lastAddQuery)
		}
		if len(lastAddBody) != 1 || lastAddBody[0].ID != 1 || lastAddBody[0].Title != "alpha" {
			t.Fatalf("body = %+v", lastAddBody)
		}
		if taskPollCount < 2 {
			t.Fatalf("taskPollCount = %d, want >= 2", taskPollCount)
		}
	})

	t.Run("index returns failed task error", func(t *testing.T) {
		mu.Lock()
		taskStatus = "failed"
		taskErrPayload = map[string]string{"message": "bad doc", "code": "invalid_document"}
		mu.Unlock()
		t.Cleanup(func() {
			mu.Lock()
			taskStatus = "succeeded"
			taskErrPayload = nil
			mu.Unlock()
		})

		err := s.Index(ctx, testDoc{ID: 2, Title: "bad"})
		if err == nil || !strings.Contains(err.Error(), "bad doc (invalid_document)") {
			t.Fatalf("Index() error = %v, want bad doc (invalid_document)", err)
		}
	})

	t.Run("delete sends ids and waits", func(t *testing.T) {
		if err := s.Delete(ctx, "1", "2"); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		mu.Lock()
		defer mu.Unlock()
		if len(lastDeleteIDs) != 2 || lastDeleteIDs[0] != "1" || lastDeleteIDs[1] != "2" {
			t.Fatalf("delete ids = %+v", lastDeleteIDs)
		}
	})

	t.Run("search decodes hits and preserves nil on empty", func(t *testing.T) {
		mu.Lock()
		searchRespBody = `{"hits":[{"id":1,"title":"alpha"},{"id":2,"title":"beta"}],"estimatedTotalHits":12,"offset":5,"limit":2,"processingTimeMs":7}`
		mu.Unlock()

		res, err := s.Search(ctx, search.Params{Query: "a", Offset: 5, Limit: 2, Filter: "id > 0"})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(res.Hits) != 2 || res.Hits[0].Title != "alpha" || res.Hits[1].ID != 2 {
			t.Fatalf("Hits = %+v", res.Hits)
		}
		if res.Total != 12 || res.Offset != 5 || res.Limit != 2 || res.Processing != 7 {
			t.Fatalf("Result metadata = %+v", res)
		}
		mu.Lock()
		if lastSearchReq["q"] != "a" || lastSearchReq["filter"] != "id > 0" {
			mu.Unlock()
			t.Fatalf("search request = %+v", lastSearchReq)
		}
		searchRespBody = `{"hits":[],"estimatedTotalHits":0,"offset":0,"limit":20,"processingTimeMs":1}`
		mu.Unlock()

		emptyRes, err := s.Search(ctx, search.Params{Query: "missing"})
		if err != nil {
			t.Fatalf("empty Search() error = %v", err)
		}
		if emptyRes.Hits != nil {
			t.Fatalf("empty Search() Hits = %#v, want nil", emptyRes.Hits)
		}
	})

	t.Run("search decode error fails call", func(t *testing.T) {
		mu.Lock()
		searchRespBody = `{"hits":[{"id":"not-an-int","title":"bad"}],"estimatedTotalHits":1,"offset":0,"limit":20,"processingTimeMs":1}`
		mu.Unlock()

		if _, err := s.Search(ctx, search.Params{Query: "bad"}); err == nil {
			t.Fatalf("expected decode error, got nil")
		}
	})

	t.Run("http error propagates", func(t *testing.T) {
		mu.Lock()
		forceStatus = http.StatusInternalServerError
		mu.Unlock()
		t.Cleanup(func() {
			mu.Lock()
			forceStatus = 0
			mu.Unlock()
		})

		if err := s.Index(ctx, testDoc{ID: 1}); err == nil {
			t.Fatalf("Index() expected error on 500")
		}
		if err := s.Delete(ctx, "1"); err == nil {
			t.Fatalf("Delete() expected error on 500")
		}
		if _, err := s.Search(ctx, search.Params{Query: "a"}); err == nil {
			t.Fatalf("Search() expected error on 500")
		}
	})
}

func TestSearcherConcurrentIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/documents"):
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":1,"indexUid":"docs","status":"enqueued"}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/tasks/"):
			_, _ = w.Write([]byte(`{"taskUid":1,"indexUid":"docs","status":"succeeded"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	mgr, err := NewServiceManager(Config{Host: srv.URL, APIKey: "k"})
	if err != nil {
		t.Fatalf("NewServiceManager() error = %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	s, err := NewSearcher[testDoc](mgr, "docs", PrimaryKey("id"))
	if err != nil {
		t.Fatalf("NewSearcher() error = %v", err)
	}

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			if err := s.Index(context.Background(), testDoc{ID: id, Title: "concurrent"}); err != nil {
				t.Errorf("Index(%d) error = %v", id, err)
			}
		}(int64(i + 1))
	}
	wg.Wait()
}
