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

// TestMatchOperation pins the endpoint-matching helper: the operation must match
// on method and route pattern together, and only listed operations may pass.
func TestMatchOperation(t *testing.T) {
	matcher := MatchOperation("/api", [][3]string{
		{"create", http.MethodPost, "/users"},
		{"read", http.MethodGet, "/users"},
		{"list", http.MethodGet, "/users/"},
	}, "create", "read")

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
