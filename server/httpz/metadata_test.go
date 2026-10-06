package httpz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/httpxmock"
	"github.com/go-sphere/httpx/stdx"
)

// matchContext builds a request whose method and reported route pattern are
// what MatchOperation keys on. The concrete path is "/raw"+fullPath, which is
// what the fail-closed warning log reports when the pattern is unavailable.
func matchContext(method, fullPath string) *httpxmock.Context {
	return httpxmock.NewRequest(method, "/raw"+fullPath, nil, httpxmock.WithFullPath(fullPath))
}

// TestMatchOperation pins the endpoint-matching helper: the operation must match
// on method and route pattern together, and only listed operations may pass.
func TestMatchOperation(t *testing.T) {
	matcher := MatchOperation("/api", [][3]string{
		{"create", http.MethodPost, "/users"},
		{"read", http.MethodGet, "/users"},
		{"list", http.MethodGet, "/users/"},
		{"download", http.MethodGet, "/files/*name"},
	}, "create", "read", "download")

	tests := []struct {
		name     string
		method   string
		fullPath string
		want     bool
	}{
		{name: "listed operation", method: http.MethodPost, fullPath: "/api/users", want: true},
		{name: "second listed operation", method: http.MethodGet, fullPath: "/api/users", want: true},
		{name: "unlisted operation", method: http.MethodGet, fullPath: "/api/users/", want: false},
		{name: "wrong method", method: http.MethodDelete, fullPath: "/api/users", want: false},
		{name: "wrong path", method: http.MethodPost, fullPath: "/api/orders", want: false},
		// Fail closed: an empty route pattern must report a hit so gated
		// middleware still runs (audit B7).
		{name: "indeterminate route fails closed", method: http.MethodGet, fullPath: "", want: true},
		// The registered pattern is the only key. The anonymous dialect used to
		// match as well, for the adapters that rewrote the pattern and reported
		// the rewrite; httpx v0.0.5 reports the registered pattern everywhere,
		// so the anonymous form is now a miss (and cannot be registered at all).
		{name: "named wildcard", method: http.MethodGet, fullPath: "/api/files/*name", want: true},
		{name: "anonymous wildcard", method: http.MethodGet, fullPath: "/api/files/*", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := matchContext(tt.method, tt.fullPath)
			if got := matcher(ctx); got != tt.want {
				t.Fatalf("match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchOperationDoesNotReadPathWhenMethodMisses(t *testing.T) {
	matcher := MatchOperation("/api", [][3]string{{"read", http.MethodGet, "/users"}}, "read")
	ctx := matchContext(http.MethodDelete, "/api/users")

	if matcher(ctx) {
		t.Fatal("unexpected match")
	}
	if ctx.CallCount("Method") != 1 || ctx.CallCount("FullPath") != 0 {
		t.Fatalf("metadata calls = method:%d fullPath:%d, want method:1 fullPath:0",
			ctx.CallCount("Method"), ctx.CallCount("FullPath"))
	}
}

// TestEndpointsToMatchesPinsJoinPaths pins the path-joining edge cases the
// endpoint table depends on: a route ending in "/" must stay distinguishable
// from the same route without it.
func TestEndpointsToMatchesPinsJoinPaths(t *testing.T) {
	got := EndpointsToMatches("/api", [][3]string{
		{"op", "GET", "/users/"},
		{"other", "GET", "/users"},
	})

	get := got["GET"]
	if len(get) != 2 {
		t.Fatalf("joined paths = %v, want both trailing-slash variants", get)
	}
	if get["/api/users/"] != "op" || get["/api/users"] != "other" {
		t.Fatalf("joined paths = %v, want the slash to be preserved", get)
	}
}

// TestEndpointsToMatchesWildcardVerbatim pins that a named wildcard is indexed
// under the path the caller registered and nothing else. The anonymous form
// ("/files/*") is no longer produced by any adapter's FullPath, so indexing it
// is dead weight; the shape check below is what keeps a future rewrite from
// sneaking back in.
func TestEndpointsToMatchesWildcardVerbatim(t *testing.T) {
	got := EndpointsToMatches("/api", [][3]string{
		{"download", "GET", "/files/*name"},
		{"static", "GET", "/assets"},
	})

	get := got["GET"]
	if get["/api/files/*name"] != "download" {
		t.Fatalf("wildcard path = %v, want /api/files/*name -> download", get)
	}
	if _, ok := get["/api/files/*"]; ok {
		t.Fatalf("anonymous wildcard indexed: %v", get)
	}
	if get["/api/assets"] != "static" {
		t.Fatalf("static path missing: %v", get)
	}
	if len(get) != 2 {
		t.Fatalf("got %d entries, want 2 (no spurious rewrites): %v", len(get), get)
	}
}

// TestMatchOperationWithNamedWildcard runs the matcher against a real engine
// rather than a fake context, because what it keys on is the engine's
// FullPath. If the router reported the wildcard in rewritten form — the leak
// v0.0.5 fixed — the wildcard route would stop matching here instead of
// silently dropping the middleware that MatchOperation gates (auth, rate
// limiting).
func TestMatchOperationWithNamedWildcard(t *testing.T) {
	matcher := MatchOperation("/api", [][3]string{{"download", http.MethodGet, "/files/*name"}}, "download")

	engine := stdx.New()
	engine.Group("/api").GET("/files/*name", func(ctx httpx.Context) error {
		if !matcher(ctx) {
			t.Errorf("MatchOperation missed %q (FullPath=%q)", ctx.Path(), ctx.FullPath())
		}
		return ctx.NoContent(http.StatusNoContent)
	})

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		t.Fatal("the engine does not support in-process requests")
	}
	resp, err := tr.Do(httptest.NewRequest(http.MethodGet, "http://example.com/api/files/a/b.txt", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
}
