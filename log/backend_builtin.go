package log

import (
	"context"
	"errors"
	"io"
)

type nopBackend struct{}

// NewNopBackend discards every log entry. Pass it to InitWithBackends to
// silence logging on purpose.
func NewNopBackend() Backend {
	return nopBackend{}
}

func (nopBackend) Log(context.Context, Level, string, ...Attr) {}

func (nopBackend) Sync() error {
	return nil
}

func (n nopBackend) With(options ...Option) Backend {
	return n
}

// MultiBackend fans each log entry out to its child backends in order.
// Create it with NewMultiBackend; it is safe for concurrent use when every
// child is.
type MultiBackend struct {
	backends []Backend
}

// Backends returns a snapshot of child backends.
func (m *MultiBackend) Backends() []Backend {
	if m == nil {
		return nil
	}
	out := make([]Backend, len(m.backends))
	copy(out, m.backends)
	return out
}

// NewMultiBackend creates a backend that writes to all provided backends.
// Nil entries are skipped. With no non-nil backend it returns the same
// discarding backend as NewNopBackend; with exactly one it returns that
// backend unwrapped; otherwise it returns a *MultiBackend.
func NewMultiBackend(backends ...Backend) Backend {
	clean := make([]Backend, 0, len(backends))
	for _, b := range backends {
		if b != nil {
			clean = append(clean, b)
		}
	}
	if len(clean) == 0 {
		return nopBackend{}
	}
	if len(clean) == 1 {
		return clean[0]
	}
	return &MultiBackend{backends: clean}
}

// Log forwards the entry to every child in order.
func (m *MultiBackend) Log(ctx context.Context, level Level, msg string, attrs ...Attr) {
	for _, b := range m.backends {
		b.Log(ctx, level, msg, attrs...)
	}
}

// Sync syncs every child, even after a failure, and joins their errors.
func (m *MultiBackend) Sync() error {
	errs := make([]error, 0, len(m.backends))
	for _, b := range m.backends {
		if err := b.Sync(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Close closes every child backend that implements io.Closer and joins their
// errors. Children without a Close are skipped. Without this, wrapping backends
// in a MultiBackend would silently drop the capability for the whole set.
// Close does not flush; call Sync first if buffered entries must reach their
// destination.
func (m *MultiBackend) Close() error {
	errs := make([]error, 0, len(m.backends))
	for _, b := range m.backends {
		closer, ok := b.(io.Closer)
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// With derives every child with options and combines the results with
// NewMultiBackend.
func (m *MultiBackend) With(options ...Option) Backend {
	next := make([]Backend, 0, len(m.backends))
	for _, b := range m.backends {
		next = append(next, b.With(options...))
	}
	return NewMultiBackend(next...)
}
