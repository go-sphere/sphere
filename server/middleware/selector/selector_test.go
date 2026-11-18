package selector

import (
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/httpxmock"
)

// stateContext returns a mock context whose StateStore holds values, as if a
// middleware above the matcher had set them.
func stateContext(values map[string]any) *httpxmock.Context {
	opts := make([]httpxmock.Option, 0, len(values))
	for k, v := range values {
		opts = append(opts, httpxmock.WithState(k, v))
	}
	return httpxmock.New(nil, opts...)
}

// TestNewContextMatcher pins the typed lookup: the value must both be present
// and of exactly the requested type.
func TestNewContextMatcher(t *testing.T) {
	matcher := NewContextMatcher[string]("role", "admin")

	tests := []struct {
		name   string
		values map[string]any
		want   bool
	}{
		{name: "matching value", values: map[string]any{"role": "admin"}, want: true},
		{name: "different value", values: map[string]any{"role": "user"}, want: false},
		{name: "missing key", values: map[string]any{}, want: false},
		{name: "wrong type", values: map[string]any{"role": 42}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := stateContext(tt.values)
			if got := matcher.Match(ctx); got != tt.want {
				t.Fatalf("match = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestLogicalCombinators pins the boolean composition of matchers, including
// the empty-list edge cases where a wrong default silently gates every request.
func TestLogicalCombinators(t *testing.T) {
	yes := MatchFunc(func(httpx.Context) bool { return true })
	no := MatchFunc(func(httpx.Context) bool { return false })
	ctx := httpxmock.New(nil)

	t.Run("not", func(t *testing.T) {
		if NewLogicalNotMatcher(yes).Match(ctx) {
			t.Error("NOT true matched")
		}
		if !NewLogicalNotMatcher(no).Match(ctx) {
			t.Error("NOT false did not match")
		}
	})

	t.Run("or", func(t *testing.T) {
		if !NewLogicalOrMatcher(no, yes).Match(ctx) {
			t.Error("OR with one true did not match")
		}
		if NewLogicalOrMatcher(no, no).Match(ctx) {
			t.Error("OR of all false matched")
		}
		if NewLogicalOrMatcher().Match(ctx) {
			t.Error("empty OR must match nothing")
		}
	})

	t.Run("and", func(t *testing.T) {
		if !NewLogicalAndMatcher(yes, yes).Match(ctx) {
			t.Error("AND of all true did not match")
		}
		if NewLogicalAndMatcher(yes, no).Match(ctx) {
			t.Error("AND with one false matched")
		}
		if !NewLogicalAndMatcher().Match(ctx) {
			t.Error("empty AND must match everything")
		}
	})
}

// TestNewSelectorMiddleware pins the gating contract: each middleware runs only
// when the matcher matches, and a skipped one hands the request downstream.
func TestNewSelectorMiddleware(t *testing.T) {
	t.Run("matching request runs the middleware", func(t *testing.T) {
		var ran int
		next := &httpxmock.Handler{}
		chain := NewSelectorMiddleware(
			MatchFunc(func(httpx.Context) bool { return true }),
			counting(&ran),
		)
		if len(chain) != 1 {
			t.Fatalf("got %d middlewares, want 1", len(chain))
		}
		if err := httpxmock.Run(httpxmock.New(nil), next.Handle, chain[0]); err != nil {
			t.Fatalf("middleware: %v", err)
		}
		if ran != 1 {
			t.Fatalf("middleware ran %d times, want 1", ran)
		}
		if !next.Called() {
			t.Error("the chain did not continue past the middleware")
		}
	})

	t.Run("non-matching request skips to the rest of the chain", func(t *testing.T) {
		var ran int
		next := &httpxmock.Handler{}
		chain := NewSelectorMiddleware(
			MatchFunc(func(httpx.Context) bool { return false }),
			counting(&ran),
		)
		if err := httpxmock.Run(httpxmock.New(nil), next.Handle, chain[0]); err != nil {
			t.Fatalf("middleware: %v", err)
		}
		if ran != 0 {
			t.Fatalf("middleware ran %d times, want 0", ran)
		}
		if !next.Called() {
			t.Error("the chain did not continue for a skipped request")
		}
	})

	t.Run("multiple middlewares keep their order", func(t *testing.T) {
		var order []int
		chain := NewSelectorMiddleware(
			MatchFunc(func(httpx.Context) bool { return true }),
			counted(1, &order),
			counted(2, &order),
		)
		if len(chain) != 2 {
			t.Fatalf("got %d middlewares, want 2", len(chain))
		}
		next := &httpxmock.Handler{}
		if err := httpxmock.Run(httpxmock.New(nil), next.Handle, chain[0]); err != nil {
			t.Fatal(err)
		}
		if err := httpxmock.Run(httpxmock.New(nil), next.Handle, chain[1]); err != nil {
			t.Fatal(err)
		}
		if len(order) != 2 || order[0] != 1 || order[1] != 2 {
			t.Fatalf("order = %v, want [1 2]", order)
		}
	})
}

// counting returns middleware that counts its runs and continues the chain.
func counting(ran *int) httpx.Middleware {
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			*ran++
			return next(ctx)
		}
	}
}

// counted returns middleware that appends its id to order and continues.
func counted(id int, order *[]int) httpx.Middleware {
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			*order = append(*order, id)
			return next(ctx)
		}
	}
}

func TestSelectorNilGuards(t *testing.T) {
	t.Parallel()

	ctx := httpxmock.New(nil)
	var nilMatch MatchFunc
	if nilMatch.Match(ctx) {
		t.Fatal("nil MatchFunc must return false")
	}
	if NewContextMatcher("k", "v").Match(nil) {
		t.Fatal("NewContextMatcher(nil) must return false")
	}
	if NewLogicalNotMatcher(nil).Match(ctx) {
		t.Fatal("NewLogicalNotMatcher(nil) must return false")
	}
	yes := MatchFunc(func(httpx.Context) bool { return true })
	no := MatchFunc(func(httpx.Context) bool { return false })
	if NewLogicalOrMatcher(nil, no).Match(ctx) {
		t.Fatal("NewLogicalOrMatcher(nil, no) must return false")
	}
	if !NewLogicalOrMatcher(nil, yes).Match(ctx) {
		t.Fatal("NewLogicalOrMatcher(nil, yes) must return true")
	}
	if NewLogicalAndMatcher(yes, nil).Match(ctx) {
		t.Fatal("NewLogicalAndMatcher(yes, nil) must return false")
	}

	var ran int
	mws := NewSelectorMiddleware(nil, nil, counting(&ran))
	if len(mws) != 1 {
		t.Fatalf("len = %d, want 1 (nil middleware skipped)", len(mws))
	}
	next := &httpxmock.Handler{}
	if err := httpxmock.Run(ctx, next.Handle, mws[0]); err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if ran != 0 || !next.Called() {
		t.Fatalf("ran=%d called=%v, want 0/true", ran, next.Called())
	}
}

func TestLogicalMatchersCloneInputSlice(t *testing.T) {
	t.Parallel()

	ctx := httpxmock.New(nil)
	yes := MatchFunc(func(httpx.Context) bool { return true })
	no := MatchFunc(func(httpx.Context) bool { return false })

	orInput := []Matcher{no}
	orMatcher := NewLogicalOrMatcher(orInput...)
	orInput[0] = yes
	if orMatcher.Match(ctx) {
		t.Fatal("NewLogicalOrMatcher reflected post-construction slice mutation")
	}

	andInput := []Matcher{yes}
	andMatcher := NewLogicalAndMatcher(andInput...)
	andInput[0] = no
	if !andMatcher.Match(ctx) {
		t.Fatal("NewLogicalAndMatcher reflected post-construction slice mutation")
	}
}
