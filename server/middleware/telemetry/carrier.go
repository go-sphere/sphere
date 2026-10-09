package telemetry

import "github.com/go-sphere/httpx"

// headerCarrier adapts the request headers to propagation.TextMapCarrier for
// extraction. It reads through Context.Header, which is case-insensitive and
// does not copy; Headers() would copy the whole map on every adapter for each
// propagator lookup. It does not implement propagation.ValuesGetter, so a
// baggage header split over several lines reads only its first line.
type headerCarrier struct{ ctx httpx.Context }

// Get returns the first value of the header k.
func (c headerCarrier) Get(k string) string { return c.ctx.Header(k) }

// Set does nothing: the server side only extracts.
func (headerCarrier) Set(string, string) {}

// Keys returns the header names present on the request.
func (c headerCarrier) Keys() []string {
	h := c.ctx.Headers()
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}
