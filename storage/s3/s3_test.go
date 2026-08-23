package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/storageerr"
)

type mockS3Server struct {
	mu         sync.Mutex
	objects    map[string][]byte
	mimes      map[string]string
	parts      map[string]map[int][]byte
	uploadMime map[string]string
}

func newMockS3Server() *mockS3Server {
	return &mockS3Server{
		objects:    make(map[string][]byte),
		mimes:      make(map[string]string),
		parts:      make(map[string]map[int][]byte),
		uploadMime: make(map[string]string),
	}
}

func (m *mockS3Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	bucket := parts[0]
	var key string
	if len(parts) == 2 {
		key = parts[1]
	}

	// Handle bucket-level requests
	if key == "" {
		if r.URL.Query().Has("location") {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<LocationConstraint>us-east-1</LocationConstraint>`))
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodPost:
		if r.URL.Query().Has("uploads") {
			uploadID := "mock-upload-id"
			m.parts[uploadID] = make(map[int][]byte)
			m.uploadMime[uploadID] = r.Header.Get("Content-Type")
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><InitiateMultipartUploadResult><Bucket>` + bucket + `</Bucket><Key>` + key + `</Key><UploadId>` + uploadID + `</UploadId></InitiateMultipartUploadResult>`))
			return
		}
		if uploadID := r.URL.Query().Get("uploadId"); uploadID != "" {
			var buf bytes.Buffer
			partMap := m.parts[uploadID]
			for i := 1; i <= len(partMap); i++ {
				buf.Write(partMap[i])
			}
			m.objects[key] = buf.Bytes()
			m.mimes[key] = m.uploadMime[uploadID]
			delete(m.parts, uploadID)
			delete(m.uploadMime, uploadID)
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><CompleteMultipartUploadResult><Bucket>` + bucket + `</Bucket><Key>` + key + `</Key><ETag>"mock-etag"</ETag></CompleteMultipartUploadResult>`))
			return
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodHead:
		data, exists := m.objects[key]
		if !exists {
			w.Header().Set("x-amz-error-code", "NoSuchKey")
			w.Header().Set("x-amz-error-message", "The specified key does not exist.")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if mimeType, ok := m.mimes[key]; ok && mimeType != "" {
			w.Header().Set("Content-Type", mimeType)
		}
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("ETag", `"mock-etag"`)
		w.WriteHeader(http.StatusOK)

	case http.MethodPut:
		if uploadID := r.URL.Query().Get("uploadId"); uploadID != "" {
			partNum, _ := strconv.Atoi(r.URL.Query().Get("partNumber"))
			body, _ := io.ReadAll(r.Body)
			if m.parts[uploadID] == nil {
				m.parts[uploadID] = make(map[int][]byte)
			}
			m.parts[uploadID][partNum] = body
			w.Header().Set("ETag", `"mock-etag"`)
			w.WriteHeader(http.StatusOK)
			return
		}

		copySource := r.Header.Get("x-amz-copy-source")
		if copySource != "" {
			srcKey := strings.TrimPrefix(copySource, "/"+bucket+"/")
			srcKey = strings.TrimPrefix(srcKey, bucket+"/")
			srcData, exists := m.objects[srcKey]
			if !exists {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`))
				return
			}
			m.objects[key] = append([]byte(nil), srcData...)
			m.mimes[key] = m.mimes[srcKey]
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<CopyObjectResult><ETag>"mock-etag"</ETag></CopyObjectResult>`))
			return
		}

		body, _ := io.ReadAll(r.Body)
		m.objects[key] = body
		m.mimes[key] = r.Header.Get("Content-Type")
		w.Header().Set("ETag", `"mock-etag"`)
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		data, exists := m.objects[key]
		if !exists {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`))
			return
		}
		if mimeType, ok := m.mimes[key]; ok && mimeType != "" {
			w.Header().Set("Content-Type", mimeType)
		}
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("ETag", `"mock-etag"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)

	case http.MethodDelete:
		delete(m.objects, key)
		delete(m.mimes, key)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusOK)
	}
}

func TestS3ClientMoveFileSelfMove(t *testing.T) {
	mock := newMockS3Server()
	server := httptest.NewServer(mock)
	defer server.Close()

	endpoint := strings.TrimPrefix(server.URL, "http://")
	client, err := NewClient(Config{
		Endpoint:        endpoint,
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		Bucket:          "bucket",
		UseSSL:          false,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	const key = "folder/file.png"

	// 1. Move missing file to same key returns ErrNotFound
	err = client.MoveFile(ctx, "nonexistent.txt", "nonexistent.txt", true)
	if !errors.Is(err, storageerr.ErrNotFound) {
		t.Fatalf("MoveFile(missing to same) error = %v, want %v", err, storageerr.ErrNotFound)
	}

	// 2. Upload file and check ContentType
	uploadedKey, err := client.UploadFile(ctx, bytes.NewBufferString("image-bytes"), key)
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}
	if uploadedKey != key {
		t.Fatalf("UploadFile() key = %q, want %q", uploadedKey, key)
	}

	// Verify ContentType was set based on extension
	mock.mu.Lock()
	if mock.mimes[key] != "image/png" {
		t.Fatalf("ContentType = %q, want %q", mock.mimes[key], "image/png")
	}
	mock.mu.Unlock()

	// 3. MoveFile to same key with overwrite=true must NOT delete the file
	err = client.MoveFile(ctx, key, key, true)
	if err != nil {
		t.Fatalf("MoveFile(same key, overwrite=true) error = %v", err)
	}

	exists, err := client.IsFileExists(ctx, key)
	if err != nil {
		t.Fatalf("IsFileExists() error = %v", err)
	}
	if !exists {
		t.Fatal("file was deleted after MoveFile onto itself")
	}

	// 4. MoveFile to different destination
	const destKey = "folder/dest.png"
	err = client.MoveFile(ctx, key, destKey, true)
	if err != nil {
		t.Fatalf("MoveFile() error = %v", err)
	}

	existsOld, _ := client.IsFileExists(ctx, key)
	if existsOld {
		t.Fatal("old key still exists after MoveFile")
	}

	existsNew, _ := client.IsFileExists(ctx, destKey)
	if !existsNew {
		t.Fatal("new key does not exist after MoveFile")
	}
}

func TestS3ClientGenerateUploadAuth(t *testing.T) {
	mock := newMockS3Server()
	server := httptest.NewServer(mock)
	defer server.Close()

	endpoint := strings.TrimPrefix(server.URL, "http://")
	client, err := NewClient(Config{
		Endpoint:        endpoint,
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		Bucket:          "mybucket",
		UseSSL:          false,
		Dir:             "uploads",
		UploadNaming:    storage.UploadNamingStrategyOriginal,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	res, err := client.GenerateUploadAuth(context.Background(), storage.UploadAuthRequest{
		FileName: "avatar.jpg",
		Dir:      "users",
	})
	if err != nil {
		t.Fatalf("GenerateUploadAuth() error = %v", err)
	}

	if res.File.Key != "uploads/users/avatar.jpg" {
		t.Fatalf("Key = %q, want %q", res.File.Key, "uploads/users/avatar.jpg")
	}
	if res.Authorization.Type != storage.UploadAuthorizationTypeURL {
		t.Fatalf("Auth Type = %q, want %q", res.Authorization.Type, storage.UploadAuthorizationTypeURL)
	}
	if res.Authorization.Method != http.MethodPut {
		t.Fatalf("Auth Method = %q, want %q", res.Authorization.Method, http.MethodPut)
	}
}
