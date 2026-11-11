package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/internal/fakes3"
	"github.com/go-sphere/sphere/storage/storageerr"
)

func newFakeClient(t *testing.T, conf Config) (*Client, *fakes3.Server) {
	t.Helper()
	fake := fakes3.New(t, "bucket")
	fake.SecretKey = "test-sk"
	conf.Endpoint = fake.Endpoint()
	conf.AccessKeyID = "test-ak"
	conf.SecretAccessKey = "test-sk"
	conf.Bucket = fake.Bucket
	client, err := NewClient(conf)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client, fake
}

func TestS3ClientMoveFileSelfMove(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	ctx := t.Context()
	const key = "folder/file.png"

	if err := client.MoveFile(ctx, "nonexistent.txt", "nonexistent.txt", true); !errors.Is(err, storageerr.ErrNotFound) {
		t.Fatalf("MoveFile(missing to same) error = %v, want ErrNotFound", err)
	}
	if _, err := client.UploadFile(ctx, strings.NewReader("image-bytes"), key); err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}
	if obj, _ := fake.Object(key); obj.MIME != "image/png" {
		t.Fatalf("ContentType = %q, want %q", obj.MIME, "image/png")
	}

	// Copy-then-delete onto itself would destroy the object.
	if err := client.MoveFile(ctx, key, key, true); err != nil {
		t.Fatalf("MoveFile(same key, overwrite=true) error = %v", err)
	}
	if _, ok := fake.Object(key); !ok {
		t.Fatal("file was deleted after MoveFile onto itself")
	}

	const destKey = "folder/dest.png"
	if err := client.MoveFile(ctx, key, destKey, true); err != nil {
		t.Fatalf("MoveFile() error = %v", err)
	}
	if _, ok := fake.Object(key); ok {
		t.Fatal("old key still exists after MoveFile")
	}
	if _, ok := fake.Object(destKey); !ok {
		t.Fatal("new key does not exist after MoveFile")
	}
}

func TestS3ClientGenerateUploadAuth(t *testing.T) {
	client, _ := newFakeClient(t, Config{Dir: "uploads", UploadNaming: storage.UploadNamingStrategyOriginal})
	res, err := client.GenerateUploadAuth(t.Context(), storage.UploadAuthRequest{FileName: "avatar.jpg", Dir: "users"})
	if err != nil {
		t.Fatalf("GenerateUploadAuth() error = %v", err)
	}
	if res.File.Key != "uploads/users/avatar.jpg" {
		t.Fatalf("Key = %q, want %q", res.File.Key, "uploads/users/avatar.jpg")
	}
	if res.Authorization.Type != storage.UploadAuthorizationTypeURL || res.Authorization.Method != http.MethodPut {
		t.Fatalf("Authorization = %s %s, want a presigned PUT URL", res.Authorization.Type, res.Authorization.Method)
	}
}

func TestS3ClientObjectLifecycle(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	ctx := t.Context()

	key, err := client.UploadFile(ctx, strings.NewReader("hello"), "/docs//a.txt")
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}
	if key != "docs/a.txt" {
		t.Fatalf("UploadFile() key = %q, want normalized %q", key, "docs/a.txt")
	}
	if obj, _ := fake.Object(key); !strings.HasPrefix(obj.MIME, "text/plain") {
		t.Fatalf("stored Content-Type = %q, want text/plain from the extension", obj.MIME)
	}

	info, err := client.StatFile(ctx, key)
	if err != nil || info.Size != 5 || !strings.HasPrefix(info.MIME, "text/plain") {
		t.Fatalf("StatFile() = %+v, %v; want size 5 text/plain", info, err)
	}
	result, err := client.DownloadFile(ctx, key)
	if err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}
	body, err := io.ReadAll(result.Reader)
	_ = result.Reader.Close()
	if err != nil || string(body) != "hello" || result.Size != 5 || !strings.HasPrefix(result.MIME, "text/plain") {
		t.Fatalf("DownloadFile() = %q size=%d mime=%q err=%v", body, result.Size, result.MIME, err)
	}

	if err := client.DeleteFile(ctx, key); err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}
	if err := client.DeleteFile(ctx, key); err != nil {
		t.Fatalf("DeleteFile(missing) error = %v, want nil (idempotent)", err)
	}
	if exists, err := client.IsFileExists(ctx, key); err != nil || exists {
		t.Fatalf("IsFileExists(deleted) = %v, %v; want false, nil", exists, err)
	}
	if _, err := client.StatFile(ctx, key); !errors.Is(err, storageerr.ErrNotFound) {
		t.Fatalf("StatFile(missing) error = %v, want ErrNotFound", err)
	}
	if _, err := client.DownloadFile(ctx, key); !errors.Is(err, storageerr.ErrNotFound) {
		t.Fatalf("DownloadFile(missing) error = %v, want ErrNotFound", err)
	}
}

func TestS3ClientUploadLocalFile(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	src := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(src, []byte("jpeg-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	key, err := client.UploadLocalFile(t.Context(), src, "img/photo.jpg")
	if err != nil {
		t.Fatalf("UploadLocalFile() error = %v", err)
	}
	obj, ok := fake.Object(key)
	if !ok || string(obj.Data) != "jpeg-bytes" || obj.MIME != "image/jpeg" {
		t.Fatalf("stored object = %q (%q), exists=%v", obj.Data, obj.MIME, ok)
	}
	if _, err := client.UploadLocalFile(t.Context(), filepath.Join(t.TempDir(), "missing"), "x.txt"); err == nil {
		t.Fatal("UploadLocalFile(missing local file) succeeded, want an error")
	}
}

func TestS3ClientCopyMoveOverwrite(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	ctx := t.Context()
	fake.Put("src.txt", []byte("source"), "text/plain")
	fake.Put("dst.txt", []byte("destination"), "text/plain")

	if err := client.CopyFile(ctx, "src.txt", "dst.txt", false); !errors.Is(err, storageerr.ErrDestExists) {
		t.Fatalf("CopyFile(no overwrite) error = %v, want ErrDestExists", err)
	}
	if err := client.MoveFile(ctx, "src.txt", "dst.txt", false); !errors.Is(err, storageerr.ErrDestExists) {
		t.Fatalf("MoveFile(no overwrite) error = %v, want ErrDestExists", err)
	}
	if obj, _ := fake.Object("dst.txt"); string(obj.Data) != "destination" {
		t.Fatalf("destination changed to %q after refused copy/move", obj.Data)
	}
	if _, ok := fake.Object("src.txt"); !ok {
		t.Fatal("refused MoveFile removed the source")
	}

	if err := client.CopyFile(ctx, "src.txt", "dst.txt", true); err != nil {
		t.Fatalf("CopyFile(overwrite) error = %v", err)
	}
	if obj, _ := fake.Object("dst.txt"); string(obj.Data) != "source" {
		t.Fatalf("destination = %q after overwrite copy, want %q", obj.Data, "source")
	}
	if err := client.MoveFile(ctx, "src.txt", "moved.txt", false); err != nil {
		t.Fatalf("MoveFile() error = %v", err)
	}
	if _, ok := fake.Object("src.txt"); ok {
		t.Fatal("source still exists after MoveFile")
	}

	if err := client.CopyFile(ctx, "missing.txt", "other.txt", true); !errors.Is(err, storageerr.ErrNotFound) {
		t.Fatalf("CopyFile(missing source) error = %v, want ErrNotFound", err)
	}
	if err := client.MoveFile(ctx, "missing.txt", "other.txt", false); !errors.Is(err, storageerr.ErrNotFound) {
		t.Fatalf("MoveFile(missing source) error = %v, want ErrNotFound", err)
	}
}

// A source above the single-request copy limit is copied part by part with
// UploadPartCopy, keeping its Content-Type; a smaller one uses CopyObject.
func TestS3ClientCopyFileLargeSourceUsesMultipartCopy(t *testing.T) {
	saved := maxCopyObjectSize
	maxCopyObjectSize = 4
	t.Cleanup(func() { maxCopyObjectSize = saved })

	client, fake := newFakeClient(t, Config{})
	ctx := t.Context()
	fake.Put("big.bin", []byte("0123456789"), "application/x-test")
	fake.Put("small.bin", []byte("0123"), "application/x-test")

	if err := client.CopyFile(ctx, "big.bin", "big-copy.bin", false); err != nil {
		t.Fatalf("CopyFile(big) error = %v", err)
	}
	want := []string{"HeadObject", "HeadObject", "CreateMultipartUpload", "UploadPartCopy", "UploadPartCopy", "UploadPartCopy", "CompleteMultipartUpload"}
	if got := fake.Ops(); !slices.Equal(got, want) {
		t.Fatalf("requests = %v, want %v", got, want)
	}
	if obj, _ := fake.Object("big-copy.bin"); string(obj.Data) != "0123456789" || obj.MIME != "application/x-test" {
		t.Fatalf("copy = %q (%s), want the source bytes and type", obj.Data, obj.MIME)
	}

	if err := client.MoveFile(ctx, "small.bin", "small-moved.bin", true); err != nil {
		t.Fatalf("MoveFile(small) error = %v", err)
	}
	want = append(want, "HeadObject", "CopyObject", "DeleteObject")
	if got := fake.Ops(); !slices.Equal(got, want) {
		t.Fatalf("requests = %v, want %v", got, want)
	}
	if obj, _ := fake.Object("small-moved.bin"); string(obj.Data) != "0123" {
		t.Fatalf("moved = %q, want %q", obj.Data, "0123")
	}
}

func TestS3ClientListFilesPaging(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	// A small server page forces minio-go through continuation tokens inside a
	// single ListFiles call.
	fake.MaxKeys = 2
	for _, key := range []string{"list/e", "list/c", "list/a", "list/d", "list/b", "other/x"} {
		fake.Put(key, []byte("x"), "")
	}
	ctx := t.Context()

	var got []string
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 10 {
			t.Fatal("ListFiles did not terminate")
		}
		keys, next, err := client.ListFiles(ctx, "/list/", cursor, 3)
		if err != nil {
			t.Fatalf("ListFiles() error = %v", err)
		}
		if len(keys) > 3 {
			t.Fatalf("ListFiles() returned %d keys, limit 3", len(keys))
		}
		got = append(got, keys...)
		if next == "" {
			break
		}
		cursor = next
	}
	if want := []string{"list/a", "list/b", "list/c", "list/d", "list/e"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("listed %v, want %v", got, want)
	}

	keys, next, err := client.ListFiles(ctx, "", "", 0)
	if err != nil || len(keys) != 6 || next != "" {
		t.Fatalf("ListFiles(all, default limit) = %v, %q, %v", keys, next, err)
	}
}

// ListFiles asks the server for one key past the page, not a full default
// page, and never above the S3 per-request maximum.
func TestS3ClientListFilesRequestsPageSize(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	for _, key := range []string{"a", "b", "c", "d"} {
		fake.Put(key, []byte("x"), "")
	}
	ctx := t.Context()

	if _, _, err := client.ListFiles(ctx, "", "", 2); err != nil {
		t.Fatalf("ListFiles(limit 2) error = %v", err)
	}
	if _, _, err := client.ListFiles(ctx, "", "", 5000); err != nil {
		t.Fatalf("ListFiles(limit 5000) error = %v", err)
	}
	if got, want := fake.ListMaxKeys(), []int{3, 1000}; !slices.Equal(got, want) {
		t.Fatalf("requested max-keys = %v, want %v", got, want)
	}
}

// closeHook runs fn when the response body is closed.
type closeHook struct {
	io.ReadCloser
	fn func()
}

func (c closeHook) Close() error {
	err := c.ReadCloser.Close()
	c.fn()
	return err
}

// cancelAfterFirstPage cancels the caller's context once the first list
// response has been fully consumed, so the client never requests page two.
type cancelAfterFirstPage struct {
	next   http.RoundTripper
	cancel context.CancelFunc
	once   sync.Once
}

func (c *cancelAfterFirstPage) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := c.next.RoundTrip(r)
	if err != nil || r.URL.Query().Get("list-type") != "2" {
		return resp, err
	}
	resp.Body = closeHook{ReadCloser: resp.Body, fn: func() { c.once.Do(c.cancel) }}
	return resp, nil
}

// TestS3ClientListFilesCanceledBetweenPages pins that a listing cut short by
// the caller's context never reads as a clean end of list: a truncated page
// with an empty cursor would let orphan cleanup treat the rest as absent.
func TestS3ClientListFilesCanceledBetweenPages(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	fake.MaxKeys = 1
	want := []string{"list/a", "list/b", "list/c"}
	for _, key := range want {
		fake.Put(key, []byte("x"), "")
	}

	canceled := 0
	for range 50 {
		ctx, cancel := context.WithCancel(t.Context())
		hook := &cancelAfterFirstPage{next: http.DefaultTransport, cancel: cancel}
		mc, err := minio.New(fake.Endpoint(), &minio.Options{
			Creds:     credentials.NewStaticV4("test-ak", "test-sk", ""),
			Transport: hook,
		})
		if err != nil {
			t.Fatalf("minio.New() error = %v", err)
		}
		client.client = mc

		keys, next, err := client.ListFiles(ctx, "list/", "", 10)
		cancel()
		switch {
		case err != nil:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("ListFiles() error = %v, want context.Canceled", err)
			}
			canceled++
		case next != "" || !slices.Equal(keys, want):
			t.Fatalf("ListFiles() = %v, %q, nil after cancel, want an error or the full listing", keys, next)
		}
	}
	if canceled == 0 {
		t.Fatal("cancellation never surfaced as an error")
	}
}

// TestS3ClientGenerateUploadAuthTTL pins the UploadAuthRequest.TTL ceiling: a
// client-supplied TTL may shorten the presigned URL but never extend it.
func TestS3ClientGenerateUploadAuthTTL(t *testing.T) {
	tests := []struct {
		name      string
		configTTL time.Duration
		reqTTL    time.Duration
		want      time.Duration
	}{
		{name: "request above ceiling is clamped", configTTL: 10 * time.Minute, reqTTL: 24 * time.Hour, want: 10 * time.Minute},
		{name: "shorter request is honoured", configTTL: 10 * time.Minute, reqTTL: time.Minute, want: time.Minute},
		{name: "zero request uses config", configTTL: 10 * time.Minute, want: 10 * time.Minute},
		{name: "unset config uses default", reqTTL: 48 * time.Hour, want: defaultUploadTTL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, fake := newFakeClient(t, Config{UploadTTL: tt.configTTL, UploadNaming: storage.UploadNamingStrategyOriginal})
			res, err := client.GenerateUploadAuth(t.Context(), storage.UploadAuthRequest{FileName: "a.png", TTL: tt.reqTTL})
			if err != nil {
				t.Fatalf("GenerateUploadAuth() error = %v", err)
			}
			u, err := url.Parse(res.Authorization.Value)
			if err != nil {
				t.Fatalf("parse presigned URL: %v", err)
			}
			if got := u.Query().Get("X-Amz-Expires"); got != strconv.Itoa(int(tt.want.Seconds())) {
				t.Fatalf("X-Amz-Expires = %s, want %d", got, int(tt.want.Seconds()))
			}

			// The presigned URL addresses the issued key on the bucket.
			resp, err := putPresigned(t, res.Authorization, res.Authorization.Headers, "png")
			if err != nil {
				t.Fatalf("PUT presigned URL: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("PUT presigned URL status = %d", resp.StatusCode)
			}
			if obj, ok := fake.Object(res.File.Key); !ok || string(obj.Data) != "png" {
				t.Fatalf("presigned upload did not land at %q", res.File.Key)
			}
		})
	}
}

// putPresigned sends body to the presigned upload URL in auth with headers.
func putPresigned(t *testing.T, auth storage.UploadAuthorization, headers map[string]string, body string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), auth.Method, auth.Value, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	return http.DefaultClient.Do(req)
}

// The presigned PUT signs the Content-Type derived from the file name, and
// types a browser would run or render as a document are signed as
// application/octet-stream, so a direct upload cannot publish them under the
// public base.
func TestS3ClientGenerateUploadAuthSignsContentType(t *testing.T) {
	tests := []struct {
		fileName string
		want     string
	}{
		{fileName: "avatar.png", want: "image/png"},
		{fileName: "clip.mp4", want: mime.TypeByExtension(".mp4")},
		{fileName: "page.html", want: "application/octet-stream"},
		{fileName: "page.htm", want: "application/octet-stream"},
		{fileName: "doc.xhtml", want: "application/octet-stream"},
		{fileName: "icon.svg", want: "application/octet-stream"},
		{fileName: "feed.xml", want: "application/octet-stream"},
		{fileName: "app.js", want: "application/octet-stream"},
		{fileName: "app.mjs", want: "application/octet-stream"},
		{fileName: "no-extension", want: "application/octet-stream"},
		{fileName: "unknown.zzz", want: "application/octet-stream"},
	}
	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			if tt.want == "" {
				t.Skip("no MIME type registered for the extension on this system")
			}
			client, fake := newFakeClient(t, Config{UploadNaming: storage.UploadNamingStrategyOriginal})
			res, err := client.GenerateUploadAuth(t.Context(), storage.UploadAuthRequest{FileName: tt.fileName})
			if err != nil {
				t.Fatalf("GenerateUploadAuth() error = %v", err)
			}
			auth := res.Authorization
			if got := auth.Headers["Content-Type"]; got != tt.want {
				t.Fatalf("Headers[Content-Type] = %q, want %q", got, tt.want)
			}
			u, err := url.Parse(auth.Value)
			if err != nil {
				t.Fatalf("parse presigned URL: %v", err)
			}
			if signed := strings.Split(u.Query().Get("X-Amz-SignedHeaders"), ";"); !slices.Contains(signed, "content-type") {
				t.Fatalf("X-Amz-SignedHeaders = %v, want content-type signed", signed)
			}

			resp, err := putPresigned(t, auth, map[string]string{"Content-Type": "text/html"}, "<script>")
			if err != nil {
				t.Fatalf("PUT with another Content-Type: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("PUT with another Content-Type status = %d, want %d", resp.StatusCode, http.StatusForbidden)
			}
			if _, ok := fake.Object(res.File.Key); ok {
				t.Fatal("PUT with another Content-Type stored the object")
			}

			resp, err = putPresigned(t, auth, auth.Headers, "body")
			if err != nil {
				t.Fatalf("PUT with the signed headers: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("PUT with the signed headers status = %d", resp.StatusCode)
			}
			if obj, ok := fake.Object(res.File.Key); !ok || obj.MIME != tt.want {
				t.Fatalf("stored MIME = %q (exists %v), want %q", obj.MIME, ok, tt.want)
			}
		})
	}
}

func TestS3ClientRejectsInvalidKeys(t *testing.T) {
	client, _ := newFakeClient(t, Config{})
	ctx := t.Context()
	const bad = "../escape"
	checks := map[string]error{
		"UploadFile":      func() error { _, err := client.UploadFile(ctx, strings.NewReader("x"), bad); return err }(),
		"UploadLocalFile": func() error { _, err := client.UploadLocalFile(ctx, "unused", bad); return err }(),
		"StatFile":        func() error { _, err := client.StatFile(ctx, bad); return err }(),
		"IsFileExists":    func() error { _, err := client.IsFileExists(ctx, bad); return err }(),
		"DownloadFile":    func() error { _, err := client.DownloadFile(ctx, bad); return err }(),
		"DeleteFile":      client.DeleteFile(ctx, bad),
		"CopyFile src":    client.CopyFile(ctx, bad, "ok.txt", true),
		"CopyFile dst":    client.CopyFile(ctx, "ok.txt", bad, true),
		"MoveFile src":    client.MoveFile(ctx, bad, "ok.txt", true),
		"MoveFile dst":    client.MoveFile(ctx, "ok.txt", bad, true),
	}
	for name, err := range checks {
		if !errors.Is(err, storageerr.ErrFileNameInvalid) {
			t.Errorf("%s error = %v, want ErrFileNameInvalid", name, err)
		}
	}
}

// TestS3ClientDownloadSurvivesOverwrite pins that the body and metadata of a
// download come from one response. Overwriting the key after DownloadFile
// returns must not break the read: the lazy minio.Object used to fetch the
// body only on first Read, with If-Match on the earlier HEAD's ETag, so it
// failed with a 412 precondition error.
func TestS3ClientDownloadSurvivesOverwrite(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	fake.Put("doc.txt", []byte("old version"), "text/plain")

	result, err := client.DownloadFile(t.Context(), "doc.txt")
	if err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}
	defer func() { _ = result.Reader.Close() }()
	fake.Put("doc.txt", []byte("new"), "text/plain")

	body, err := io.ReadAll(result.Reader)
	if err != nil {
		t.Fatalf("read after overwrite: %v", err)
	}
	if string(body) != "old version" || result.Size != int64(len(body)) {
		t.Fatalf("body = %q size = %d, want the version the metadata described", body, result.Size)
	}
}

// TestS3ClientFailedUploadKeepsExistingObject pins that an upload whose reader
// fails leaves the key's previous object in place: minio-go aborts the
// multipart upload, which must not delete what is already stored.
func TestS3ClientFailedUploadKeepsExistingObject(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	fake.Put("doc.txt", []byte("original"), "text/plain")

	failing := io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(errors.New("stream dropped")))
	if _, err := client.UploadFile(t.Context(), failing, "doc.txt"); err == nil {
		t.Fatal("UploadFile(failing reader) succeeded, want an error")
	}
	if obj, ok := fake.Object("doc.txt"); !ok || string(obj.Data) != "original" {
		t.Fatalf("stored object = %q exists=%v after failed upload, want %q", obj.Data, ok, "original")
	}
}

func TestNewClientPartSize(t *testing.T) {
	client, _ := newFakeClient(t, Config{})
	if client.config.PartSize != defaultPartSize {
		t.Fatalf("PartSize = %d, want default %d", client.config.PartSize, defaultPartSize)
	}
	if _, err := NewClient(Config{Endpoint: "localhost:9000", PartSize: minPartSize - 1}); err == nil {
		t.Fatal("NewClient accepted a part size below the S3 minimum")
	}
}

// UploadFile sends a body that ends within PartSize as one PutObject and only
// switches to a multipart upload for a longer one.
func TestS3ClientUploadFileRequests(t *testing.T) {
	multipart := func(parts int) []string {
		ops := []string{"CreateMultipartUpload"}
		for range parts {
			ops = append(ops, "UploadPart")
		}
		return append(ops, "CompleteMultipartUpload")
	}
	tests := []struct {
		name string
		size int
		want []string
	}{
		{name: "empty", size: 0, want: []string{"PutObject"}},
		{name: "small", size: 10, want: []string{"PutObject"}},
		{name: "exactly one part", size: minPartSize, want: []string{"PutObject"}},
		{name: "one byte over a part", size: minPartSize + 1, want: multipart(2)},
		{name: "several parts", size: 2*minPartSize + 7, want: multipart(3)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, fake := newFakeClient(t, Config{PartSize: minPartSize})
			data := bytes.Repeat([]byte("0123456789"), tt.size/10+1)[:tt.size]
			// io.MultiReader hides the length, as a request body would.
			key, err := client.UploadFile(t.Context(), io.MultiReader(bytes.NewReader(data)), "dir/file.txt")
			if err != nil {
				t.Fatalf("UploadFile() error = %v", err)
			}
			if key != "dir/file.txt" {
				t.Fatalf("UploadFile() key = %q", key)
			}
			if got := fake.Ops(); !slices.Equal(got, tt.want) {
				t.Fatalf("requests = %v, want %v", got, tt.want)
			}
			obj, ok := fake.Object("dir/file.txt")
			if !ok || !bytes.Equal(obj.Data, data) {
				t.Fatalf("stored object mismatch: ok=%v len=%d, want len %d", ok, len(obj.Data), len(data))
			}
			if want := mime.TypeByExtension(".txt"); obj.MIME != want {
				t.Fatalf("stored MIME = %q, want %q", obj.MIME, want)
			}
		})
	}
}

// A reader that fails after the first part aborts the multipart upload and
// leaves the stored object alone.
func TestS3ClientFailedMultipartUploadAborts(t *testing.T) {
	client, fake := newFakeClient(t, Config{PartSize: minPartSize})
	fake.Put("doc.txt", []byte("original"), "text/plain")

	failing := io.MultiReader(bytes.NewReader(make([]byte, minPartSize+10)), iotest.ErrReader(errors.New("stream dropped")))
	if _, err := client.UploadFile(t.Context(), failing, "doc.txt"); err == nil {
		t.Fatal("UploadFile(failing reader) succeeded, want an error")
	}
	if want := []string{"CreateMultipartUpload", "UploadPart", "AbortMultipartUpload"}; !slices.Equal(fake.Ops(), want) {
		t.Fatalf("requests = %v, want %v", fake.Ops(), want)
	}
	if obj, ok := fake.Object("doc.txt"); !ok || string(obj.Data) != "original" {
		t.Fatalf("stored object = %q exists=%v after failed upload, want %q", obj.Data, ok, "original")
	}
}

// UploadFile has no length for its reader, so the configured PartSize is what
// bounds the per-upload buffer.
func TestS3ClientUploadFileUsesPartSize(t *testing.T) {
	client, fake := newFakeClient(t, Config{PartSize: minPartSize})
	data := bytes.Repeat([]byte("x"), 2*minPartSize+1)
	// io.MultiReader hides the length, as a request body would.
	if _, err := client.UploadFile(t.Context(), io.MultiReader(bytes.NewReader(data)), "big.bin"); err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}
	if got := fake.LargestPart(); got != minPartSize {
		t.Fatalf("largest part = %d, want %d", got, minPartSize)
	}
	obj, ok := fake.Object("big.bin")
	if !ok || !bytes.Equal(obj.Data, data) {
		t.Fatalf("stored object mismatch: ok=%v len=%d", ok, len(obj.Data))
	}
}
