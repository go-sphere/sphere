package httpz

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/httpxmock"
)

// uploadContext builds a context over a real multipart request carrying one
// file under the "file" field, so FormFile parses exactly what the framework
// would hand the wrapper.
func uploadContext(tb testing.TB, filename string, content []byte) *httpxmock.Context {
	tb.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		tb.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		tb.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return httpxmock.New(req)
}

// TestWithFormFileReader pins the upload validation chain: missing form field,
// oversize file, and disallowed extension must all reject before the handler
// runs; a valid upload must reach it with the original filename.
func TestWithFormFileReader(t *testing.T) {
	t.Run("missing form field surfaces the error", func(t *testing.T) {
		// A request with no multipart body is what a missing field looks like.
		ctx := httpxmock.New(nil)
		handler := WithFormFileReader(func(httpx.Context, io.ReadSeekCloser, string) (string, error) {
			t.Fatal("handler must not run")
			return "", nil
		})

		// The wrapper renders the error through the standard error response
		// path instead of returning it, classified as a client error.
		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ctx.StatusCode() != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", ctx.StatusCode(), http.StatusBadRequest)
		}
	})

	t.Run("oversize file is rejected", func(t *testing.T) {
		ctx := uploadContext(t, "big.bin", bytes.Repeat([]byte("x"), 100))
		handler := WithFormFileReader(func(httpx.Context, io.ReadSeekCloser, string) (string, error) {
			t.Fatal("handler must not run")
			return "", nil
		}, WithFormMaxSize(10))

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ctx.StatusCode() != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", ctx.StatusCode(), http.StatusBadRequest)
		}
		resp := jsonBody[ErrorResponse](t, ctx)
		if want := "File size exceeds maximum allowed size: big.bin"; resp.Message != want {
			t.Fatalf("Message = %q, want %q", resp.Message, want)
		}
	})

	t.Run("disallowed extension is rejected", func(t *testing.T) {
		ctx := uploadContext(t, "script.sh", []byte("rm -rf /"))
		handler := WithFormFileReader(func(httpx.Context, io.ReadSeekCloser, string) (string, error) {
			t.Fatal("handler must not run")
			return "", nil
		}, WithFormAllowExtensions(".jpg", ".png"))

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if ctx.StatusCode() != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", ctx.StatusCode(), http.StatusBadRequest)
		}
	})

	t.Run("valid upload reaches the handler", func(t *testing.T) {
		content := []byte("photo bytes")
		ctx := uploadContext(t, "photo.JPG", content)
		handler := WithFormFileReader(func(_ httpx.Context, f io.ReadSeekCloser, name string) (string, error) {
			all, err := io.ReadAll(f)
			if err != nil {
				t.Fatalf("reading the upload: %v", err)
			}
			if string(all) != string(content) {
				t.Errorf("content = %q, want %q", all, content)
			}
			if name != "photo.JPG" {
				t.Errorf("filename = %q, want photo.JPG", name)
			}
			return name, nil
		}, WithFormAllowExtensions(".jpg"))

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
	})
}

// TestWithFormFileBytesPinsBytePath pins that the byte variant reads the whole
// upload into memory, and that its extension check is case-insensitive as
// documented.
func TestWithFormFileBytesPinsBytePath(t *testing.T) {
	t.Run("reads the upload", func(t *testing.T) {
		ctx := uploadContext(t, "photo.png", []byte("photo bytes"))
		handler := WithFormFileBytes(func(_ httpx.Context, data []byte, name string) (int, error) {
			if string(data) != "photo bytes" {
				t.Errorf("data = %q, want the upload contents", data)
			}
			if name != "photo.png" {
				t.Errorf("filename = %q, want photo.png", name)
			}
			return len(data), nil
		})

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
	})

	t.Run("extension check is case-insensitive", func(t *testing.T) {
		ctx := uploadContext(t, "photo.JPG", []byte("x"))
		handler := WithFormFileBytes(func(httpx.Context, []byte, string) (int, error) {
			return 1, nil
		}, WithFormAllowExtensions(".jpg"))

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
	})

	t.Run("extension check works without leading dot", func(t *testing.T) {
		ctx := uploadContext(t, "photo.png", []byte("x"))
		handler := WithFormFileBytes(func(httpx.Context, []byte, string) (int, error) {
			return 1, nil
		}, WithFormAllowExtensions("png", "jpg"))

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
	})

	t.Run("empty extension list allows everything", func(t *testing.T) {
		ctx := uploadContext(t, "weird.bin", []byte("x"))
		handler := WithFormFileBytes(func(httpx.Context, []byte, string) (int, error) {
			return 1, nil
		}, WithFormAllowExtensions())

		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
	})
}

// TestWithFormOptionsDefaults pins the documented defaults so a config change
// here is deliberate.
func TestWithFormOptionsDefaults(t *testing.T) {
	opts := newWithFormOptions()
	if opts.maxSize != 10*1024*1024 {
		t.Errorf("default maxSize = %d, want 10MB", opts.maxSize)
	}
	if opts.fileFormKey != "file" {
		t.Errorf("default form key = %q, want file", opts.fileFormKey)
	}
	if opts.allowExtensions != nil {
		t.Errorf("default extension allowlist = %v, want nil (allow all)", opts.allowExtensions)
	}

	custom := newWithFormOptions(WithFormMaxSize(1), WithFormFileKey("avatar"), WithFormAllowExtensions(".PNG"))
	if custom.maxSize != 1 || custom.fileFormKey != "avatar" {
		t.Fatalf("custom options not applied: %+v", custom)
	}
	if _, ok := custom.allowExtensions[".png"]; !ok {
		t.Errorf("extensions must be lowercased: %v", custom.allowExtensions)
	}

}
