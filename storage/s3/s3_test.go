package s3

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/internal/fakes3"
	"github.com/go-sphere/sphere/storage/storageerr"
)

func newFakeClient(t *testing.T, conf Config) (*Client, *fakes3.Server) {
	t.Helper()
	fake := fakes3.New(t, "bucket")
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
			req, err := http.NewRequestWithContext(t.Context(), res.Authorization.Method, res.Authorization.Value, strings.NewReader("png"))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
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

// UploadFile has no length for its reader, so the configured PartSize is what
// bounds minio-go's per-upload buffer.
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
