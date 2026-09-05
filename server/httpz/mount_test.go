package httpz

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/fiberx"
	"github.com/go-sphere/httpx/ginx"
)

// plainRegistrar implements httpx.Registrar without StdHandlerMounter to pin
// the fail-loud path of MountStdAll.
type plainRegistrar struct{}

func (plainRegistrar) Handle(method, path string, h httpx.Handler) {}
func (plainRegistrar) Any(path string, h httpx.Handler)            {}
func (plainRegistrar) Static(prefix, root string)                  {}
func (plainRegistrar) StaticFS(prefix string, fs fs.FS)            {}

func TestMountStdAllUnsupportedRegistrar(t *testing.T) {
	err := MountStdAll(plainRegistrar{}, "/x", http.NotFoundHandler())
	if err == nil {
		t.Fatal("MountStdAll on a registrar without StdHandlerMounter must fail loud")
	}
}

// TestMountStdAllAcrossAdapters mounts one net/http handler through the httpx
// abstraction on ginx and fiberx and asserts equivalent responses through
// in-process dispatch, including the default method set and an explicit
// method restriction.
func TestMountStdAllAcrossAdapters(t *testing.T) {
	engines := map[string]func() httpx.Engine{
		"ginx":   func() httpx.Engine { return ginx.New() },
		"fiberx": func() httpx.Engine { return fiberx.New() },
	}
	handler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("X-Std", "yes")
		rw.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(rw, r.Method+" "+r.URL.Path)
	})

	for name, newEngine := range engines {
		t.Run(name, func(t *testing.T) {
			engine := newEngine()
			root := engine.Group("")
			if err := MountStdAll(root, "/std", handler); err != nil {
				t.Fatalf("MountStdAll default methods: %v", err)
			}
			if err := MountStdAll(root, "/only-get", handler, http.MethodGet); err != nil {
				t.Fatalf("MountStdAll explicit method: %v", err)
			}

			tr, ok := httpx.AsTestRequester(engine)
			if !ok {
				t.Fatalf("%s engine does not support in-process test dispatch", name)
			}
			do := func(method, target string) *http.Response {
				t.Helper()
				resp, err := tr.Do(httptest.NewRequest(method, "http://example.com"+target, nil))
				if err != nil {
					t.Fatalf("%s %s: %v", method, target, err)
				}
				return resp
			}

			for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
				resp := do(method, "/std")
				body, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK || string(body) != method+" /std" {
					t.Fatalf("%s /std = %d %q", method, resp.StatusCode, body)
				}
				if resp.Header.Get("X-Std") != "yes" {
					t.Fatalf("%s /std missing std handler header", method)
				}
			}

			resp := do(http.MethodPost, "/only-get")
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Fatalf("POST /only-get = %d, want non-200 (mounted for GET only)", resp.StatusCode)
			}
		})
	}
}
