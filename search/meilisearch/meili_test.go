package meilisearch

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-sphere/sphere/search"
	"github.com/meilisearch/meilisearch-go"
)

type Article struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

/*
docker run -it --rm \
  -p 7700:7700 \
  -v $(pwd)/meili_data:/meili_data \
  getmeili/meilisearch:latest
*/

func TestSearcher(t *testing.T) {
	manager, err := NewServiceManager(Config{
		Host:   "http://localhost:7700",
		APIKey: "8IdbIxzCm86BaD8ZkT4SGv9vaipY1Ax7i0sz_Qv8wTI",
	})
	if err != nil {
		t.Fatalf("Failed to create service manager: %v", err)
	}
	// The constructor no longer probes the server, so check availability here and
	// skip when Meilisearch is not reachable (e.g. in CI without a running server).
	if _, err := manager.service.HealthWithContext(context.Background()); err != nil {
		t.Skipf("Meilisearch server not available, skipping test: %v", err)
	}
	searcher, err := NewSearcher[Article](manager, "articles", PrimaryKey("id"))
	if err != nil {
		t.Errorf("Failed to create searcher: %v", err)
		return
	}

	ctx := context.Background()

	articles := []Article{
		{
			ID:      1,
			Title:   "hello world",
			Content: "This is a test article",
		},
		{
			ID:      2,
			Title:   "goodbye world",
			Content: "This is another test article",
		},
	}
	err = searcher.Index(ctx, articles...)
	if err != nil {
		t.Errorf("Indexing articles failed: %v", err)
		return
	}

	result, err := searcher.Search(ctx, search.Params{
		Query: "hello",
	})
	if err != nil {
		t.Errorf("Search failed: %v", err)
		return
	}

	if len(result.Hits) != 1 || result.Hits[0].Title != "hello world" {
		t.Errorf("Expected to find 1 article with title 'hello world', got %d articles", len(result.Hits))
		return
	}
	if result.Total != 1 {
		t.Errorf("Expected total hits to be 1, got %d", result.Total)
		return
	}

	err = searcher.Delete(ctx, "2")
	if err != nil {
		t.Errorf("Deleting article failed: %v", err)
		return
	}

	found, err := searcher.Search(ctx, search.Params{
		Query: "goodbye world",
	})
	if err != nil {
		t.Errorf("Search after deletion failed: %v", err)
		return
	}

	if len(found.Hits) != 0 {
		t.Errorf("Expected to find no articles after deletion, got %d articles", len(found.Hits))
		return
	}
}

func TestTaskError(t *testing.T) {
	var failedWithError meilisearch.Task
	if err := json.Unmarshal([]byte(`{"status":"failed","taskUid":2,"error":{"message":"document invalid","code":"invalid_document"}}`), &failedWithError); err != nil {
		t.Fatalf("failed to unmarshal task: %v", err)
	}

	tests := []struct {
		name        string
		task        *meilisearch.Task
		wantErr     bool
		expectedErr string
	}{
		{
			name:    "nil task returns nil",
			task:    nil,
			wantErr: false,
		},
		{
			name: "succeeded task returns nil",
			task: &meilisearch.Task{
				TaskUID: 1,
				Status:  meilisearch.TaskStatusSucceeded,
			},
			wantErr: false,
		},
		{
			name:        "failed task with error message and code",
			task:        &failedWithError,
			wantErr:     true,
			expectedErr: "meilisearch: task 2 failed: document invalid (invalid_document)",
		},
		{
			name: "failed task without error message",
			task: &meilisearch.Task{
				TaskUID: 3,
				Status:  meilisearch.TaskStatusFailed,
			},
			wantErr:     true,
			expectedErr: "meilisearch: task 3 failed",
		},
		{
			name: "canceled task",
			task: &meilisearch.Task{
				TaskUID: 4,
				Status:  meilisearch.TaskStatusCanceled,
			},
			wantErr:     true,
			expectedErr: "meilisearch: task 4 canceled",
		},
		{
			name: "enqueued task treated as error",
			task: &meilisearch.Task{
				TaskUID: 5,
				Status:  meilisearch.TaskStatusEnqueued,
			},
			wantErr:     true,
			expectedErr: "meilisearch: task 5 enqueued",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := taskError(tc.task)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if err.Error() != tc.expectedErr {
					t.Fatalf("expected error message %q, got %q", tc.expectedErr, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
			}
		})
	}
}

// TestSearchTaskErrorAllStatuses verifies error formatting across all Meilisearch task statuses,
// error structures (with message, without message, with code, with empty code), and edge cases.
func TestSearchTaskErrorAllStatuses(t *testing.T) {
	statuses := []struct {
		name        string
		taskJSON    string
		wantErr     bool
		expectedMsg string
	}{
		{
			name:        "nil task",
			taskJSON:    "",
			wantErr:     false,
			expectedMsg: "",
		},
		{
			name:        "succeeded status",
			taskJSON:    `{"taskUid":100,"status":"succeeded"}`,
			wantErr:     false,
			expectedMsg: "",
		},
		{
			name:        "failed with message and code",
			taskJSON:    `{"taskUid":100,"status":"failed","error":{"message":"primary key is invalid","code":"invalid_primary_key"}}`,
			wantErr:     true,
			expectedMsg: "meilisearch: task 100 failed: primary key is invalid (invalid_primary_key)",
		},
		{
			name:        "failed without error message or code",
			taskJSON:    `{"taskUid":100,"status":"failed"}`,
			wantErr:     true,
			expectedMsg: "meilisearch: task 100 failed",
		},
		{
			name:        "failed with message only",
			taskJSON:    `{"taskUid":100,"status":"failed","error":{"message":"unknown index error"}}`,
			wantErr:     true,
			expectedMsg: "meilisearch: task 100 failed: unknown index error ()",
		},
		{
			name:        "canceled status with message and code",
			taskJSON:    `{"taskUid":100,"status":"canceled","error":{"message":"task was canceled by admin","code":"task_canceled"}}`,
			wantErr:     true,
			expectedMsg: "meilisearch: task 100 canceled: task was canceled by admin (task_canceled)",
		},
		{
			name:        "canceled status without message",
			taskJSON:    `{"taskUid":100,"status":"canceled"}`,
			wantErr:     true,
			expectedMsg: "meilisearch: task 100 canceled",
		},
		{
			name:        "enqueued status (incomplete wait)",
			taskJSON:    `{"taskUid":100,"status":"enqueued"}`,
			wantErr:     true,
			expectedMsg: "meilisearch: task 100 enqueued",
		},
		{
			name:        "processing status (incomplete wait)",
			taskJSON:    `{"taskUid":100,"status":"processing"}`,
			wantErr:     true,
			expectedMsg: "meilisearch: task 100 processing",
		},
		{
			name:        "unknown custom status",
			taskJSON:    `{"taskUid":100,"status":"unknown_status"}`,
			wantErr:     true,
			expectedMsg: "meilisearch: task 100 unknown_status",
		},
	}

	for _, tc := range statuses {
		t.Run(tc.name, func(t *testing.T) {
			var task *meilisearch.Task
			if tc.taskJSON != "" {
				var target meilisearch.Task
				if err := json.Unmarshal([]byte(tc.taskJSON), &target); err != nil {
					t.Fatalf("json.Unmarshal failed: %v", err)
				}
				task = &target
			}

			err := taskError(task)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if err.Error() != tc.expectedMsg {
					t.Fatalf("error message mismatch: got %q, want %q", err.Error(), tc.expectedMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("expected nil error, got: %v", err)
				}
			}
		})
	}
}

// TestSearchTaskErrorJSONDeserialization verifies that real JSON payloads produced by Meilisearch
// unmarshal correctly into meilisearch.Task and yield the expected error message format.
func TestSearchTaskErrorJSONDeserialization(t *testing.T) {
	jsonPayloads := []struct {
		name        string
		jsonRaw     string
		wantErr     bool
		expectedMsg string
	}{
		{
			name:        "success payload",
			jsonRaw:     `{"taskUid":42,"status":"succeeded","type":"documentAdditionOrUpdate","details":{"receivedDocuments":10,"indexedDocuments":10}}`,
			wantErr:     false,
			expectedMsg: "",
		},
		{
			name:        "failed payload with meili error details",
			jsonRaw:     `{"taskUid":99,"status":"failed","type":"documentAdditionOrUpdate","error":{"message":"Document with id 1 is missing primary key","code":"missing_primary_key","type":"invalid_request","link":"https://docs.meilisearch.com/errors#missing_primary_key"}}`,
			wantErr:     true,
			expectedMsg: "meilisearch: task 99 failed: Document with id 1 is missing primary key (missing_primary_key)",
		},
		{
			name:        "canceled payload",
			jsonRaw:     `{"taskUid":105,"status":"canceled","type":"taskCancelation"}`,
			wantErr:     true,
			expectedMsg: "meilisearch: task 105 canceled",
		},
	}

	for _, tc := range jsonPayloads {
		t.Run(tc.name, func(t *testing.T) {
			var task meilisearch.Task
			if err := json.Unmarshal([]byte(tc.jsonRaw), &task); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}
			err := taskError(&task)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if err.Error() != tc.expectedMsg {
					t.Fatalf("expected %q, got %q", tc.expectedMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("expected nil error, got: %v", err)
				}
			}
		})
	}
}
