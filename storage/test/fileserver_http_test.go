package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/server/httpz"
	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/fileserver"
)

// newEngine returns a stdx engine and the http.Handler it is. stdx is the
// adapter these tests drive because it needs no framework and its Engine
// serves net/http directly, so the routes the file server registers are
// matched by a real router rather than by a stand-in written here.
func newEngine(t *testing.T) (httpx.Engine, http.Handler) {
	t.Helper()
	engine := stdx.New()
	handler, ok := engine.(http.Handler)
	if !ok {
		t.Fatalf("stdx engine %T is not an http.Handler", engine)
	}
	return engine, handler
}

func TestFileServerUploadAndDownloadOverHTTP(t *testing.T) {
	engine, handler := newEngine(t)
	server := httptest.NewServer(handler)
	defer server.Close()

	tokenCache := memory.NewByteCache()
	t.Cleanup(func() { _ = tokenCache.Close() })
	memStorage := newInMemoryStorage(t)

	fileServer, err := fileserver.NewCDNAdapter(
		fileserver.Config{
			PutBase:      server.URL + "/upload",
			GetBase:      server.URL + "/files",
			UploadNaming: storage.UploadNamingStrategyOriginal,
		},
		tokenCache,
		memStorage,
	)
	if err != nil {
		t.Fatalf("NewCDNAdapter() error = %v", err)
	}
	fileServer.RegisterFileUploader(engine.Group("/upload"))
	fileServer.RegisterFileDownloader(engine.Group("/files"))

	tokenData, err := fileServer.GenerateUploadAuth(context.Background(), storage.UploadAuthRequest{
		FileName: "avatar.txt",
		Dir:      "users",
	})
	if err != nil {
		t.Fatalf("GenerateUploadAuth() error = %v", err)
	}
	uploadURL := tokenData.Authorization.Value
	key := tokenData.File.Key
	downloadURL := tokenData.File.URL
	if key != "users/avatar.txt" {
		t.Fatalf("key = %q, want %q", key, "users/avatar.txt")
	}

	payload := []byte("hello over http")
	putReq, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new PUT request: %v", err)
	}
	putResp, err := server.Client().Do(putReq)
	if err != nil {
		t.Fatalf("PUT upload request failed: %v", err)
	}
	defer func() { _ = putResp.Body.Close() }()
	if putResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(putResp.Body)
		t.Fatalf("upload status = %d, want %d, body = %s", putResp.StatusCode, http.StatusOK, string(body))
	}
	var putResult httpz.DataResponse[fileserver.UploadResult]
	if err = json.NewDecoder(putResp.Body).Decode(&putResult); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if putResult.Data.Key != key {
		t.Fatalf("upload response key = %q, want %q", putResult.Data.Key, key)
	}

	getResp, err := server.Client().Get(downloadURL)
	if err != nil {
		t.Fatalf("GET download request failed: %v", err)
	}
	defer func() { _ = getResp.Body.Close() }()
	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		t.Fatalf("download status = %d, want %d, body = %s", getResp.StatusCode, http.StatusOK, string(body))
	}
	all, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("read download body: %v", err)
	}
	if string(all) != string(payload) {
		t.Fatalf("download body = %q, want %q", string(all), string(payload))
	}
}

// TestFileServerDownloadIsNotRenderable pins the response headers that keep an
// uploaded document from executing as script on the origin serving GetBase.
// The content type is derived from the key's extension, so a .html or .svg
// upload comes back as text/html or image/svg+xml; without nosniff and an
// attachment disposition the browser renders it and any embedded script runs
// with the origin's cookies. The repo's own file service wires PutBase and
// GetBase to the same base URL, so that origin is routinely the application's.
func TestFileServerDownloadIsNotRenderable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		key     string
		payload string
	}{
		{name: "html", key: "payload.html", payload: "<script>alert(document.domain)</script>"},
		{name: "svg", key: "payload.svg", payload: `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine, handler := newEngine(t)
			server := httptest.NewServer(handler)
			defer server.Close()

			tokenCache := memory.NewByteCache()
			t.Cleanup(func() { _ = tokenCache.Close() })

			fileServer, err := fileserver.NewCDNAdapter(
				fileserver.Config{
					PutBase:      server.URL + "/upload",
					GetBase:      server.URL + "/files",
					UploadNaming: storage.UploadNamingStrategyOriginal,
				},
				tokenCache,
				newInMemoryStorage(t),
			)
			if err != nil {
				t.Fatalf("NewCDNAdapter() error = %v", err)
			}
			fileServer.RegisterFileDownloader(engine.Group("/files"))

			if _, err = fileServer.UploadFile(context.Background(), strings.NewReader(tc.payload), tc.key); err != nil {
				t.Fatalf("UploadFile() error = %v", err)
			}

			resp, err := server.Client().Get(server.URL + "/files/" + tc.key)
			if err != nil {
				t.Fatalf("GET download request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want %q", got, "nosniff")
			}
			disposition := resp.Header.Get("Content-Disposition")
			if !strings.HasPrefix(disposition, "attachment") {
				t.Errorf("Content-Disposition = %q, want an attachment disposition", disposition)
			}
		})
	}
}

// TestFileServerInlineDownloadOptOut pins that the attachment default is
// escapable for deployments that serve GetBase from a session-free origin.
func TestFileServerInlineDownloadOptOut(t *testing.T) {
	engine, handler := newEngine(t)
	server := httptest.NewServer(handler)
	defer server.Close()

	tokenCache := memory.NewByteCache()
	t.Cleanup(func() { _ = tokenCache.Close() })

	fileServer, err := fileserver.NewCDNAdapter(
		fileserver.Config{
			PutBase:      server.URL + "/upload",
			GetBase:      server.URL + "/files",
			UploadNaming: storage.UploadNamingStrategyOriginal,
		},
		tokenCache,
		newInMemoryStorage(t),
		fileserver.WithInlineDownload(),
	)
	if err != nil {
		t.Fatalf("NewCDNAdapter() error = %v", err)
	}
	fileServer.RegisterFileDownloader(engine.Group("/files"))

	if _, err = fileServer.UploadFile(context.Background(), strings.NewReader("body"), "photo.png"); err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}

	resp, err := server.Client().Get(server.URL + "/files/photo.png")
	if err != nil {
		t.Fatalf("GET download request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if got := resp.Header.Get("Content-Disposition"); got != "" {
		t.Errorf("Content-Disposition = %q, want none when inline is enabled", got)
	}
	// nosniff is not part of the opt-out: it never prevents a legitimate
	// content type from rendering, it only blocks type upgrades.
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", got, "nosniff")
	}
}

type unknownSizeStorage struct{ storage.Storage }

func (s unknownSizeStorage) DownloadFile(ctx context.Context, key string) (storage.DownloadResult, error) {
	result, err := s.Storage.DownloadFile(ctx, key)
	result.Size = -1
	return result, err
}

func TestFileServerDownloadUnknownSize(t *testing.T) {
	engine, handler := newEngine(t)
	tokens := memory.NewByteCache()
	t.Cleanup(func() { _ = tokens.Close() })
	store := unknownSizeStorage{newInMemoryStorage(t)}
	if _, err := store.UploadFile(t.Context(), strings.NewReader("payload"), "a.txt"); err != nil {
		t.Fatal(err)
	}
	adapter, err := fileserver.NewCDNAdapter(fileserver.Config{
		PutBase: "http://localhost/upload", GetBase: "http://localhost/files",
	}, tokens, store)
	if err != nil {
		t.Fatal(err)
	}
	adapter.RegisterFileDownloader(engine.Group("/files"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/a.txt", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "payload" {
		t.Fatalf("download = %d %q, want 200 payload", rec.Code, rec.Body.String())
	}
}

// TestFileServerMaxUploadSize pins WithMaxUploadSize over a real server: a
// declared Content-Length over the limit is refused with 413 without spending
// the token, a chunked body that streams past the limit is cut off with 413
// and leaves no object, and a body at the limit is stored.
func TestFileServerMaxUploadSize(t *testing.T) {
	const limit = 16
	engine, handler := newEngine(t)
	server := httptest.NewServer(handler)
	defer server.Close()

	tokenCache := memory.NewByteCache()
	t.Cleanup(func() { _ = tokenCache.Close() })
	memStorage := newInMemoryStorage(t)
	fileServer, err := fileserver.NewCDNAdapter(
		fileserver.Config{
			PutBase:      server.URL + "/upload",
			GetBase:      server.URL + "/files",
			UploadNaming: storage.UploadNamingStrategyOriginal,
		},
		tokenCache,
		memStorage,
		fileserver.WithMaxUploadSize(limit),
	)
	if err != nil {
		t.Fatalf("NewCDNAdapter() error = %v", err)
	}
	fileServer.RegisterFileUploader(engine.Group("/upload"))

	ctx := context.Background()
	put := func(url string, body io.Reader) int {
		t.Helper()
		req, err := http.NewRequest(http.MethodPut, url, body)
		if err != nil {
			t.Fatalf("new PUT request: %v", err)
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatalf("PUT: %v", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	auth := func(name string) storage.UploadAuthResult {
		t.Helper()
		res, err := fileServer.GenerateUploadAuth(ctx, storage.UploadAuthRequest{FileName: name})
		if err != nil {
			t.Fatalf("GenerateUploadAuth: %v", err)
		}
		return res
	}

	declared := auth("declared.bin")
	if got := put(declared.Authorization.Value, bytes.NewReader(make([]byte, limit+1))); got != http.StatusRequestEntityTooLarge {
		t.Fatalf("declared oversize status = %d, want 413", got)
	}
	if got := put(declared.Authorization.Value, bytes.NewReader(make([]byte, limit))); got != http.StatusOK {
		t.Fatalf("retry at the limit with the same token = %d, want 200", got)
	}

	streamed := auth("streamed.bin")
	// Hiding the reader's length makes the client send a chunked body with no
	// Content-Length, so only the streaming check can catch it.
	chunked := io.MultiReader(bytes.NewReader(make([]byte, 4*limit)))
	if got := put(streamed.Authorization.Value, chunked); got != http.StatusRequestEntityTooLarge {
		t.Fatalf("streamed oversize status = %d, want 413", got)
	}
	if ok, err := memStorage.IsFileExists(ctx, streamed.File.Key); err != nil || ok {
		t.Fatalf("truncated object stored: exists=%v err=%v", ok, err)
	}
}
