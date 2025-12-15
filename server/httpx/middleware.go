package httpx

type MiddlewareOrder int

const (
	MiddlewareBeforeNext MiddlewareOrder = iota
	MiddlewareAfterNext
)

// Middleware wraps a Handler with cross-cutting behavior (logging, tracing, etc).
type Middleware func(Handler) Handler

// MiddlewareChain manages ordered middleware application.
type MiddlewareChain struct {
	chain []Middleware
}

// NewMiddlewareChain creates a chain with the given middleware.
func NewMiddlewareChain(m ...Middleware) *MiddlewareChain {
	return &MiddlewareChain{chain: compactMiddleware(m)}
}

// Clone returns a copy of the chain that can be mutated independently.
func (m *MiddlewareChain) Clone() *MiddlewareChain {
	return &MiddlewareChain{chain: m.Middlewares()}
}

// Use appends middleware in call order.
func (m *MiddlewareChain) Use(mm ...Middleware) {
	m.chain = append(m.chain, compactMiddleware(mm)...)
}

// Middlewares returns a defensive copy of the underlying chain.
func (m *MiddlewareChain) Middlewares() []Middleware {
	return append([]Middleware(nil), m.chain...)
}

// Then applies the chain to a Handler in order (first middleware runs first).
func (m *MiddlewareChain) Then(h Handler) Handler {
	return ApplyMiddleware(h, m.chain...)
}

// Wrap collapses the chain into single middleware.
func (m *MiddlewareChain) Wrap() Middleware {
	return ComposeMiddleware(m.chain...)
}

// ApplyMiddleware wraps h with the provided middleware in declaration order.
func ApplyMiddleware(h Handler, m ...Middleware) Handler {
	if len(m) == 0 {
		return h
	}
	for i := len(m) - 1; i >= 0; i-- {
		if m[i] != nil {
			h = m[i](h)
		}
	}
	return h
}

// ComposeMiddleware flattens multiple middleware into one.
func ComposeMiddleware(m ...Middleware) Middleware {
	chain := compactMiddleware(m)
	if len(chain) == 0 {
		return func(next Handler) Handler { return next }
	}
	return func(next Handler) Handler {
		return ApplyMiddleware(next, chain...)
	}
}

func compactMiddleware(m []Middleware) []Middleware {
	if len(m) == 0 {
		return nil
	}
	out := make([]Middleware, 0, len(m))
	for _, mw := range m {
		if mw != nil {
			out = append(out, mw)
		}
	}
	return out
}
