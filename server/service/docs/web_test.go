package docs

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
	"github.com/swaggo/swag"
)

func TestWebLifecycleContract(t *testing.T) {
	tasktest.AssertLifecycleContract(t, func() task.Task {
		return NewWebServer(Config{Address: "127.0.0.1:0"})
	})
}

func TestNewIndexHandler(t *testing.T) {
	t.Parallel()

	body := []byte("<html><body>Docs Index</body></html>")
	handler := newIndexHandler(body)

	t.Run("GET / returns index HTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/html" {
			t.Fatalf("Content-Type = %q, want text/html", ct)
		}
		if rec.Body.String() != string(body) {
			t.Fatalf("body = %q, want %q", rec.Body.String(), string(body))
		}
	})

	t.Run("HEAD / returns 200 with empty body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("HEAD body length = %d, want 0", rec.Body.Len())
		}
	})

	t.Run("GET /other returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/other", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})
}

func TestNewHandlerRejectsNilTargetSpec(t *testing.T) {
	t.Parallel()

	web := NewWebServer(Config{Targets: []Target{{Address: "https://api.example.com"}}})
	handler, err := web.newHandler()
	if err == nil {
		t.Fatal("newHandler() error = nil, want template execution error")
	}
	if handler != nil {
		t.Fatalf("newHandler() handler = %v, want nil", handler)
	}
}

func TestWithCORS(t *testing.T) {
	t.Parallel()

	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	corsHandler := withCORS(baseHandler)

	t.Run("OPTIONS preflight returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()
		corsHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
			t.Fatalf("Allow-Origin = %q, want http://localhost:3000", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Fatalf("Allow-Credentials = %q, want true", got)
		}
	})

	t.Run("request with Origin mirrors Origin and sets Credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()
		corsHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
			t.Fatalf("Allow-Origin = %q, want https://example.com", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Fatalf("Allow-Credentials = %q, want true", got)
		}
	})

	t.Run("request without Origin sets wildcard without Credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		rec := httptest.NewRecorder()
		corsHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("Allow-Origin = %q, want *", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Fatalf("Allow-Credentials = %q, want empty", got)
		}
	})
}

func TestRegisterTargetAndProxying(t *testing.T) {
	t.Parallel()

	var receivedHost, receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("backend payload"))
	}))
	defer backend.Close()

	spec := &swag.Spec{
		InfoInstanceName: "TargetV1",
	}

	mux := http.NewServeMux()
	if err := registerTarget(mux, spec, backend.URL); err != nil {
		t.Fatalf("registerTarget error: %v", err)
	}

	// Test proxy path routing
	req := httptest.NewRequest(http.MethodGet, "/targetv1/api/users", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "backend payload" {
		t.Fatalf("body = %q, want backend payload", rec.Body.String())
	}
	if receivedPath != "/users" {
		t.Fatalf("received path = %q, want /users", receivedPath)
	}

	backendURL, _ := url.Parse(backend.URL)
	if receivedHost != backendURL.Host {
		t.Fatalf("received host = %q, want %q", receivedHost, backendURL.Host)
	}
}
