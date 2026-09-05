package httpz

import (
	"net/http"
	"testing"
)

// matchFakeContext adds request metadata overrides used by MatchOperation.
type matchFakeContext struct {
	httpxContext
	method        string
	fullPath      string
	methodCalls   int
	fullPathCalls int
}

func (m *matchFakeContext) Method() string {
	m.methodCalls++
	return m.method
}

func (m *matchFakeContext) FullPath() string {
	m.fullPathCalls++
	return m.fullPath
}

// Path backs the fail-closed warning log, which reports the raw request path
// when the route pattern is unavailable.
func (m *matchFakeContext) Path() string {
	return "/raw" + m.fullPath
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
		// echox/fiberx rewrite "/files/*name" to "/files/*" at registration
		// time; both wildcard dialects must resolve to the operation.
		{name: "named wildcard", method: http.MethodGet, fullPath: "/api/files/*name", want: true},
		{name: "anonymous wildcard", method: http.MethodGet, fullPath: "/api/files/*", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &matchFakeContext{method: tt.method, fullPath: tt.fullPath}
			if got := matcher(ctx); got != tt.want {
				t.Fatalf("match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchOperationDoesNotReadPathWhenMethodMisses(t *testing.T) {
	matcher := MatchOperation("/api", [][3]string{{"read", http.MethodGet, "/users"}}, "read")
	ctx := &matchFakeContext{method: http.MethodDelete, fullPath: "/api/users"}

	if matcher(ctx) {
		t.Fatal("unexpected match")
	}
	if ctx.methodCalls != 1 || ctx.fullPathCalls != 0 {
		t.Fatalf("metadata calls = method:%d fullPath:%d, want method:1 fullPath:0", ctx.methodCalls, ctx.fullPathCalls)
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

// TestEndpointsToMatchesWildcardDualForm pins the dual indexing of trailing
// named wildcards: both the verbatim and the anonymous dialect must resolve,
// and non-trailing or bare asterisks must stay untouched.
func TestEndpointsToMatchesWildcardDualForm(t *testing.T) {
	got := EndpointsToMatches("/api", [][3]string{
		{"download", "GET", "/files/*name"},
		{"static", "GET", "/assets"},
	})

	get := got["GET"]
	if get["/api/files/*name"] != "download" || get["/api/files/*"] != "download" {
		t.Fatalf("wildcard forms = %v, want both /api/files/*name and /api/files/*", get)
	}
	if get["/api/assets"] != "static" {
		t.Fatalf("static path missing: %v", get)
	}
	if len(get) != 3 {
		t.Fatalf("got %d entries, want 3 (no spurious rewrites): %v", len(get), get)
	}
}

func TestAnonymousWildcardPath(t *testing.T) {
	cases := map[string]string{
		"/files/*name":  "/files/*",
		"/files/*":      "/files/*",
		"/files":        "/files",
		"/a/*x/tail":    "/a/*x/tail", // wildcard not in final segment: untouched
		"/foo*bar":      "/foo*bar",   // not preceded by '/': untouched
		"*root":         "*root",
		"/":             "/",
		"":              "",
		"/deep/a/*rest": "/deep/a/*",
		"/files/*name/": "/files/*name/", // trailing slash after wildcard: untouched
	}
	for in, want := range cases {
		if got := anonymousWildcardPath(in); got != want {
			t.Fatalf("anonymousWildcardPath(%q) = %q, want %q", in, got, want)
		}
	}
}
