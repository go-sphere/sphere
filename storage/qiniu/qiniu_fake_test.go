package qiniu

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/internal/fakeqiniu"
	"github.com/go-sphere/sphere/storage/storageerr"
)

// newFakeClient runs the driver, through the real SDK, against the in-process
// Kodo emulation.
func newFakeClient(t *testing.T, conf Config) (*Client, *fakeqiniu.Server) {
	t.Helper()
	fake := fakeqiniu.New(t, "bucket")
	conf.AccessKey = fake.AccessKey
	conf.SecretKey = fake.SecretKey
	conf.Bucket = fake.Bucket
	if conf.PublicBase == "" {
		conf.PublicBase = "https://cdn.example.com"
	}
	client, err := NewClient(conf)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client, fake
}

func TestClientObjectLifecycle(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	ctx := t.Context()

	key, err := client.UploadFile(ctx, strings.NewReader("hello"), "/docs//a.txt")
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}
	if key != "docs/a.txt" {
		t.Fatalf("UploadFile() key = %q, want normalized %q", key, "docs/a.txt")
	}
	if obj, ok := fake.Object(key); !ok || string(obj.Data) != "hello" {
		t.Fatalf("stored object = %q, exists=%v", obj.Data, ok)
	}

	if exists, err := client.IsFileExists(ctx, key); err != nil || !exists {
		t.Fatalf("IsFileExists() = %v, %v; want true, nil", exists, err)
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
	// Kodo answers 612 for a missing key; the contract makes delete idempotent.
	if err := client.DeleteFile(ctx, key); err != nil {
		t.Fatalf("DeleteFile(missing) error = %v, want nil", err)
	}
	if exists, err := client.IsFileExists(ctx, key); err != nil || exists {
		t.Fatalf("IsFileExists(deleted) = %v, %v; want false, nil", exists, err)
	}
	if _, err := client.StatFile(ctx, key); !errors.Is(err, storageerr.ErrNotFound) {
		t.Fatalf("StatFile(missing) error = %v, want ErrNotFound (612)", err)
	}
	if _, err := client.DownloadFile(ctx, key); !errors.Is(err, storageerr.ErrNotFound) {
		t.Fatalf("DownloadFile(missing) error = %v, want ErrNotFound (HTTP 404)", err)
	}
}

func TestClientUploadLocalFile(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	src := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(src, []byte("jpeg-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	key, err := client.UploadLocalFile(t.Context(), src, "img//photo.jpg")
	if err != nil {
		t.Fatalf("UploadLocalFile() error = %v", err)
	}
	if obj, ok := fake.Object(key); key != "img/photo.jpg" || !ok || string(obj.Data) != "jpeg-bytes" {
		t.Fatalf("UploadLocalFile() key=%q stored=%q exists=%v", key, obj.Data, ok)
	}
	if _, err := client.UploadLocalFile(t.Context(), filepath.Join(t.TempDir(), "missing"), "x.txt"); err == nil {
		t.Fatal("UploadLocalFile(missing local file) succeeded, want an error")
	}
}

func TestClientCopyMoveOverwrite(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	ctx := t.Context()
	fake.Put("src.txt", []byte("source"), "text/plain")
	fake.Put("dst.txt", []byte("destination"), "text/plain")

	// Kodo answers 614 when the destination exists and force is off.
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
	fake.Put("dst.txt", []byte("destination"), "text/plain")
	if err := client.MoveFile(ctx, "src.txt", "dst.txt", true); err != nil {
		t.Fatalf("MoveFile(overwrite) error = %v", err)
	}
	if obj, _ := fake.Object("dst.txt"); string(obj.Data) != "source" {
		t.Fatalf("destination = %q after overwrite move, want %q", obj.Data, "source")
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

func TestClientListFilesPaging(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
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
		// A leading separator on the prefix is tolerated.
		keys, next, err := client.ListFiles(ctx, "/list/", cursor, 2)
		if err != nil {
			t.Fatalf("ListFiles() error = %v", err)
		}
		if len(keys) > 2 {
			t.Fatalf("ListFiles() returned %d keys, limit 2", len(keys))
		}
		got = append(got, keys...)
		if next == "" {
			break
		}
		// The fake's marker is opaque; it must be passed back verbatim.
		if strings.HasPrefix(next, "list/") {
			t.Fatalf("next cursor %q looks like a key, want the backend marker", next)
		}
		cursor = next
	}
	if want := "list/a,list/b,list/c,list/d,list/e"; strings.Join(got, ",") != want {
		t.Fatalf("listed %v, want %s", got, want)
	}

	// Out-of-range limits fall back to Kodo's maximum instead of erroring.
	for _, limit := range []int{0, -1, 5000} {
		keys, next, err := client.ListFiles(ctx, "", "", limit)
		if err != nil || len(keys) != 6 || next != "" {
			t.Fatalf("ListFiles(limit=%d) = %v, %q, %v", limit, keys, next, err)
		}
	}
}

// TestClientGenerateUploadAuthTTL pins the UploadAuthRequest.TTL ceiling on the
// issued token itself: a client-supplied TTL may shorten the token's deadline
// but never extend it past Config.UploadTTL.
func TestClientGenerateUploadAuthTTL(t *testing.T) {
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
		{name: "sub-second request rounds up to one second", configTTL: time.Minute, reqTTL: time.Millisecond, want: time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, fake := newFakeClient(t, Config{UploadTTL: tt.configTTL, Dir: "up", UploadNaming: storage.UploadNamingStrategyOriginal})
			before := time.Now().Unix()
			res, err := client.GenerateUploadAuth(t.Context(), storage.UploadAuthRequest{FileName: "a.png", Dir: "u", TTL: tt.reqTTL})
			after := time.Now().Unix()
			if err != nil {
				t.Fatalf("GenerateUploadAuth() error = %v", err)
			}
			policy, err := fake.VerifyUploadToken(res.Authorization.Value)
			if err != nil {
				t.Fatalf("issued token rejected: %v", err)
			}
			ttl := int64(tt.want.Seconds())
			if policy.Deadline < before+ttl || policy.Deadline > after+ttl {
				t.Fatalf("token deadline = now%+ds, want now+%ds", policy.Deadline-before, ttl)
			}
			if res.File.Key != "up/u/a.png" || policy.Scope != "bucket:up/u/a.png" || policy.InsertOnly != 1 {
				t.Fatalf("key=%q policy=%+v, want key-scoped insert-only token", res.File.Key, policy)
			}
			if policy.MimeLimit != DefaultMimeLimit {
				t.Fatalf("mimeLimit = %q, want %q", policy.MimeLimit, DefaultMimeLimit)
			}
			if res.File.URL != "https://cdn.example.com/up/u/a.png" || res.Authorization.Type != storage.UploadAuthorizationTypeToken {
				t.Fatalf("result = %+v", res)
			}
		})
	}
}

// TestClientUploadAuthTokenIsUsable posts a client-side upload with an issued
// token, the way a browser would, and checks the policy the token carries.
func TestClientUploadAuthTokenIsUsable(t *testing.T) {
	client, fake := newFakeClient(t, Config{UploadNaming: storage.UploadNamingStrategyOriginal})
	res, err := client.GenerateUploadAuth(t.Context(), storage.UploadAuthRequest{FileName: "a.png"})
	if err != nil {
		t.Fatalf("GenerateUploadAuth() error = %v", err)
	}
	post := func(key, contentType, body string) int {
		t.Helper()
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("token", res.Authorization.Value)
		_ = mw.WriteField("key", key)
		part, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": {`form-data; name="file"; filename="a.png"`},
			"Content-Type":        {contentType},
		})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(part, body)
		_ = mw.Close()
		req, err := http.NewRequestWithContext(t.Context(), res.Authorization.Method, fake.UploadURL(), &buf)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("post upload: %v", err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	if code := post(res.File.Key, "text/html", "<script>"); code == http.StatusOK {
		t.Fatal("default MimeLimit accepted text/html")
	}
	if code := post("elsewhere.png", "image/png", "png"); code == http.StatusOK {
		t.Fatal("token accepted a key outside its scope")
	}
	if code := post(res.File.Key, "image/png", "png"); code != http.StatusOK {
		t.Fatalf("upload with issued token status = %d, want 200", code)
	}
	if obj, ok := fake.Object(res.File.Key); !ok || string(obj.Data) != "png" {
		t.Fatalf("token upload did not land at %q", res.File.Key)
	}
	// InsertOnly: the same token cannot replace what it uploaded.
	if code := post(res.File.Key, "image/png", "other"); code == http.StatusOK {
		t.Fatal("insert-only token overwrote an existing object")
	}
}

func TestClientRejectsInvalidKeys(t *testing.T) {
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
	if _, err := client.GenerateUploadAuth(ctx, storage.UploadAuthRequest{FileName: "a.png", Dir: "../.."}); err == nil {
		t.Error("GenerateUploadAuth(dir escaping the root) succeeded, want an error")
	}
}

// TestClientUploadOverwrites pins that server-side uploads replace an existing
// key. With a bucket-only token scope Kodo treats the upload as insert-only and
// rejects different content for an existing key with 614.
func TestClientUploadOverwrites(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	ctx := t.Context()
	if _, err := client.UploadFile(ctx, strings.NewReader("v1"), "doc.txt"); err != nil {
		t.Fatalf("UploadFile(v1) error = %v", err)
	}
	if _, err := client.UploadFile(ctx, strings.NewReader("v2"), "doc.txt"); err != nil {
		t.Fatalf("UploadFile(v2 over v1) error = %v", err)
	}
	src := filepath.Join(t.TempDir(), "v3.txt")
	if err := os.WriteFile(src, []byte("v3"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := client.UploadLocalFile(ctx, src, "doc.txt"); err != nil {
		t.Fatalf("UploadLocalFile(v3 over v2) error = %v", err)
	}
	if obj, _ := fake.Object("doc.txt"); string(obj.Data) != "v3" {
		t.Fatalf("stored = %q, want %q", obj.Data, "v3")
	}
}

// TestClientMoveToSameKey pins that moving a key onto itself is a no-op for
// both overwrite values, and reports ErrNotFound for a missing key.
func TestClientMoveToSameKey(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	ctx := t.Context()
	if err := client.MoveFile(ctx, "missing.txt", "missing.txt", false); !errors.Is(err, storageerr.ErrNotFound) {
		t.Fatalf("MoveFile(missing onto itself) error = %v, want ErrNotFound", err)
	}
	fake.Put("self.txt", []byte("keep"), "text/plain")
	for _, overwrite := range []bool{false, true} {
		if err := client.MoveFile(ctx, "self.txt", "/self.txt", overwrite); err != nil {
			t.Fatalf("MoveFile(onto itself, overwrite=%v) error = %v", overwrite, err)
		}
		if obj, ok := fake.Object("self.txt"); !ok || string(obj.Data) != "keep" {
			t.Fatalf("object changed after self-move (overwrite=%v): %q exists=%v", overwrite, obj.Data, ok)
		}
	}
}

// TestClientDownloadReportsTruncation pins that a download failing after its
// headers surfaces as a read error. The SDK closes its pipe with a nil error
// before reporting the real one, so without a length check the caller saw a
// clean EOF on a short (here: empty) body. Replacing the object between the
// SDK's HEAD and its GET makes the GET fail with an ETag mismatch.
func TestClientDownloadReportsTruncation(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	fake.Put("doc.txt", []byte("0123456789"), "text/plain")
	fake.SetIOHook(func(r *http.Request) {
		if r.Method == http.MethodGet {
			fake.Put("doc.txt", []byte("abcdefghij"), "text/plain")
		}
	})

	result, err := client.DownloadFile(t.Context(), "doc.txt")
	if err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}
	defer func() { _ = result.Reader.Close() }()
	body, err := io.ReadAll(result.Reader)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadAll() = %q, %v; want io.ErrUnexpectedEOF for a body short of Size %d", body, err, result.Size)
	}
}

// TestClientDownloadFallsBackToStat covers the path where the download
// response carries no Content-Type and the driver asks Stat for it.
func TestClientDownloadFallsBackToStat(t *testing.T) {
	client, fake := newFakeClient(t, Config{})
	fake.Put("blob", []byte("raw"), "")

	result, err := client.DownloadFile(t.Context(), "blob")
	if err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}
	body, err := io.ReadAll(result.Reader)
	_ = result.Reader.Close()
	if err != nil || string(body) != "raw" || result.Size != 3 {
		t.Fatalf("DownloadFile() = %q size=%d err=%v", body, result.Size, err)
	}
}
