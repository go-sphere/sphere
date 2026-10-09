package ops

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/server/httpz"
)

func get(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func TestHealthz(t *testing.T) {
	h := Handler()
	if w := get(t, h, http.MethodGet, HealthzPath); w.Code != 200 || w.Body.String() != "ok" {
		t.Fatalf("GET: %d %q", w.Code, w.Body.String())
	}
	if w := get(t, h, http.MethodHead, HealthzPath); w.Code != 200 || w.Body.Len() != 0 {
		t.Fatalf("HEAD: %d %q", w.Code, w.Body.String())
	}
	if w := get(t, h, http.MethodPost, HealthzPath); w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") == "" {
		t.Fatalf("POST: %d %v", w.Code, w.Header())
	}
}

// TestPprofOffByDefault pins the opt-in: without WithPprof no pprof path is
// served, and with it the standard endpoints are.
func TestPprofOffByDefault(t *testing.T) {
	for _, p := range []string{PprofPrefix, PprofPrefix + "cmdline", PprofPrefix + "heap", PprofPrefix + "profile"} {
		if w := get(t, Handler(), http.MethodGet, p); w.Code != http.StatusNotFound {
			t.Errorf("default %s = %d, want 404", p, w.Code)
		}
	}
	h := Handler(WithPprof())
	for _, p := range []string{PprofPrefix, PprofPrefix + "cmdline", PprofPrefix + "heap", PprofPrefix + "goroutine?debug=1"} {
		if w := get(t, h, http.MethodGet, p); w.Code != http.StatusOK {
			t.Errorf("pprof %s = %d, want 200", p, w.Code)
		}
	}
	if w := get(t, h, http.MethodGet, HealthzPath); w.Code != 200 {
		t.Errorf("healthz with pprof = %d", w.Code)
	}
}

// TestHandlerLeavesDefaultServeMuxAlone pins that Handler registers on its own
// mux: the business-facing http.DefaultServeMux gets no /healthz route from it.
func TestHandlerLeavesDefaultServeMuxAlone(t *testing.T) {
	Handler(WithPprof())
	r := httptest.NewRequest(http.MethodGet, HealthzPath, nil)
	if _, pattern := http.DefaultServeMux.Handler(r); pattern != "" {
		t.Fatalf("DefaultServeMux serves %q", pattern)
	}
}

func TestHealthzMountsOnEngine(t *testing.T) {
	app := stdx.New()
	if err := httpz.MountStdAll(app.Group(""), HealthzPath, HealthzHandler(), http.MethodGet); err != nil {
		t.Fatal(err)
	}
	if w := get(t, app.(http.Handler), http.MethodGet, HealthzPath); w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestServerLifecycle(t *testing.T) {
	s := NewServer("127.0.0.1:0", WithPprof())
	done := make(chan error, 1)
	go func() { done <- s.Start(t.Context()) }()
	deadline := time.Now().Add(5 * time.Second)
	for s.Addr() == "" {
		if time.Now().After(deadline) {
			t.Fatal("server never bound")
		}
		time.Sleep(time.Millisecond)
	}
	resp, err := http.Get("http://" + s.Addr() + HealthzPath)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "ok" {
		t.Fatalf("healthz: %d %q", resp.StatusCode, body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Start after Stop = %v, want nil", err)
	}
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("second Stop = %v", err)
	}
	if s.Identifier() != "ops" {
		t.Fatal("identifier")
	}
}

func TestServerStartTwice(t *testing.T) {
	s := NewServer("127.0.0.1:0")
	go func() { _ = s.Start(t.Context()) }()
	for s.Addr() == "" {
		time.Sleep(time.Millisecond)
	}
	defer func() { _ = s.Stop(context.Background()) }()
	if err := s.Start(t.Context()); err == nil {
		t.Fatal("second Start succeeded")
	}
}
