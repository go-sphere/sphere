package log

import "context"

type nopBackend struct{}

func NewNopBackend() Backend {
	return nopBackend{}
}

func (nopBackend) Log(context.Context, Level, string, ...Attr) {}

func (nopBackend) Sync() error {
	return nil
}

func (n nopBackend) With(options ...Option) Backend {
	_ = options
	return n
}

// MultiBackend fan-outs each log entry to all configured backends.
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

func (m *MultiBackend) Log(ctx context.Context, level Level, msg string, attrs ...Attr) {
	for _, b := range m.backends {
		b.Log(ctx, level, msg, attrs...)
	}
}

func (m *MultiBackend) Sync() error {
	var firstErr error
	for _, b := range m.backends {
		if err := b.Sync(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *MultiBackend) With(options ...Option) Backend {
	next := make([]Backend, 0, len(m.backends))
	for _, b := range m.backends {
		next = append(next, b.With(options...))
	}
	return NewMultiBackend(next...)
}
