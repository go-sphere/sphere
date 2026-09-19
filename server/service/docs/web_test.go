package docs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
	"github.com/swaggo/swag"
)

func TestWebLifecycleContract(t *testing.T) {
	tasktest.AssertLifecycleContract(t, func() task.Task {
		return NewWebServer(Config{Address: "127.0.0.1:0"})
	})
}

func TestWeb_Identifier(t *testing.T) {
	t.Parallel()
	web := NewWebServer(Config{})
	if got := web.Identifier(); got != "docs" {
		t.Fatalf("web.Identifier() = %q, want docs", got)
	}
}

// TestRegisterTargetRejectsInvalidURLs pins the fail-fast validation: url.Parse
// accepts "localhost:8080" (scheme "localhost", empty host) and bare paths, and
// without the check the misconfiguration only surfaced as per-request
// "unsupported protocol scheme" proxy failures.
func TestRegisterTargetRejectsInvalidURLs(t *testing.T) {
	t.Parallel()

	spec := &swag.Spec{InfoInstanceName: "TargetV1"}
	for _, target := range []string{"localhost:8080", "/api", "ftp://example.com", "https://"} {
		if err := registerTarget(http.NewServeMux(), spec, target); err == nil {
			t.Errorf("registerTarget(%q) = nil, want an error", target)
		}
	}
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
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("Allow-Origin = %q, want *", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Fatalf("Allow-Credentials = %q, want unset", got)
		}
	})

	// Never credentialed: reflecting the Origin together with
	// Access-Control-Allow-Credentials would let any website drive
	// cookie-bearing requests through the docs proxy.
	t.Run("request with Origin stays wildcard without Credentials", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()
		corsHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("Allow-Origin = %q, want *", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Fatalf("Allow-Credentials = %q, want unset", got)
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

// TestEngineModeServesDocs pins the httpx-mounted mode: index, API reverse
// proxy, and the relaxed CORS wrapper must behave like the standalone
// http.Server mode.
func TestEngineModeServesDocs(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("backend:" + r.URL.Path))
	}))
	defer backend.Close()

	engine := stdx.New()
	web, err := NewWebServerWithEngine(Config{
		Targets: []Target{{Address: backend.URL, Spec: &swag.Spec{InfoInstanceName: "EngineV1"}}},
	}, engine)
	if err != nil {
		t.Fatalf("NewWebServerWithEngine: %v", err)
	}
	if web.Identifier() != "docs" {
		t.Fatalf("Identifier = %q", web.Identifier())
	}

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		t.Fatal("the engine does not support in-process dispatch")
	}
	do := func(method, target string, origin string) *http.Response {
		t.Helper()
		req := httptest.NewRequest(method, "http://example.com"+target, nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := tr.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, target, err)
		}
		return resp
	}

	// Index page at "/" through the catch-all mount.
	resp := do(http.MethodGet, "/", "")
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, body=%q", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/html" {
		t.Fatalf("GET / Content-Type = %q", ct)
	}
	if !strings.Contains(string(body), "EngineV1") {
		t.Fatalf("index does not list target: %q", body)
	}

	// API reverse proxy strips the instance prefix.
	resp = do(http.MethodGet, "/enginev1/api/users", "")
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "backend:/users" {
		t.Fatalf("proxy = %d %q, want 200 backend:/users", resp.StatusCode, body)
	}

	// CORS preflight is handled by the docs wrapper.
	resp = do(http.MethodOptions, "/enginev1/api/users", "http://localhost:3000")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("OPTIONS preflight = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin = %q, want *", got)
	}
}

func TestNewWebServerWithEngineNilEnginePanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil engine")
		}
	}()
	_, _ = NewWebServerWithEngine(Config{}, nil)
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

// fakeSwagger is a swag.Swagger whose document the test controls. The library's
// *swag.Spec reads its document from an unexported field, so a document that
// can be served at all has to come from a type of our own.
type fakeSwagger string

func (f fakeSwagger) ReadDoc() string { return string(f) }

// TestSwaggerDocJSONRewritesWithoutMutatingSpec pins both halves of the docs
// proxy contract. The document the UI fetches has to point at the local proxy
// prefix, otherwise "Try it out" calls the target's real host and base path.
// The caller's *swag.Spec must come back unchanged: it is normally the same
// pointer the generated docs package registered in the process-wide swag
// registry, and the API service serves its own /swagger/doc.json from it.
func TestSwaggerDocJSONRewritesWithoutMutatingSpec(t *testing.T) {
	const instance = "DocJSONTargetV1"
	if swag.GetSwagger(instance) == nil {
		swag.Register(instance, fakeSwagger(`{"swagger":"2.0","host":"api.internal","basePath":"/real/base","info":{"title":"t"}}`))
	}

	spec := &swag.Spec{InfoInstanceName: instance, Description: "described", Version: "v1"}
	web := NewWebServer(Config{Targets: []Target{{Address: "https://api.example.com", Spec: spec}}})
	handler, err := web.newHandler()
	if err != nil {
		t.Fatalf("newHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/docjsontargetv1/doc/swagger/doc.json", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("doc.json status = %d, want 200", rec.Code)
	}

	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("doc.json is not JSON: %v", err)
	}
	if got := doc["basePath"]; got != "/docjsontargetv1/api" {
		t.Fatalf("basePath = %v, want /docjsontargetv1/api", got)
	}
	if got := doc["host"]; got != "" {
		t.Fatalf("host = %v, want empty", got)
	}

	if spec.BasePath != "" || spec.Host != "" || spec.Description != "described" {
		t.Fatalf("caller spec was mutated: %+v", spec)
	}
}
