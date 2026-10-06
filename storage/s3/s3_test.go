package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/internal/fakes3"
	"github.com/go-sphere/sphere/storage/storageerr"
)

func TestS3ClientMoveFileSelfMove(t *testing.T) {
	fake := fakes3.New(t, "bucket")
	client, err := NewClient(Config{
		Endpoint:        fake.Endpoint(),
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
	if obj, _ := fake.Object(key); obj.MIME != "image/png" {
		t.Fatalf("ContentType = %q, want %q", obj.MIME, "image/png")
	}

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

	existsOld, err := client.IsFileExists(ctx, key)
	if err != nil {
		t.Fatalf("IsFileExists(old key) error = %v", err)
	}
	if existsOld {
		t.Fatal("old key still exists after MoveFile")
	}

	existsNew, err := client.IsFileExists(ctx, destKey)
	if err != nil {
		t.Fatalf("IsFileExists(new key) error = %v", err)
	}
	if !existsNew {
		t.Fatal("new key does not exist after MoveFile")
	}
}

func TestS3ClientGenerateUploadAuth(t *testing.T) {
	fake := fakes3.New(t, "mybucket")
	client, err := NewClient(Config{
		Endpoint:        fake.Endpoint(),
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

func newFakeClient(t *testing.T, conf Config) (*Client, *fakes3.Server) {
	t.Helper()
	fake := fakes3.New(t, "bucket")
	conf.Endpoint = fake.Endpoint()
	conf.AccessKeyID = "test-ak"
	conf.SecretAccessKey = "test-sk"
	conf.Bucket = "bucket"
	client, err := NewClient(conf)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client, fake
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
