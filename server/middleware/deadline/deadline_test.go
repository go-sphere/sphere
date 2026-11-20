package deadline

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/httpxmock"
	"github.com/go-sphere/sphere/server/httpz"
)

func request(ctx context.Context) *httpxmock.Context {
	return httpxmock.NewRequest(http.MethodGet, "/slow", nil, httpxmock.WithContext(ctx))
}

// TestNewSetsDeadline pins that the handler's context carries the deadline,
// that an earlier parent deadline is kept, and that a non-positive duration
// leaves the context alone.
func TestNewSetsDeadline(t *testing.T) {
	t.Parallel()

	deadlineSeen := func(ctx context.Context, d time.Duration) (time.Time, bool) {
		var (
			at time.Time
			ok bool
		)
		err := httpxmock.Run(request(ctx), func(c httpx.Context) error {
			at, ok = c.Context().Deadline()
			return nil
		}, New(d))
		if err != nil {
			t.Fatalf("chain: %v", err)
		}
		return at, ok
	}

	start := time.Now()
	at, ok := deadlineSeen(t.Context(), time.Minute)
	if !ok || at.Before(start.Add(time.Minute)) || at.After(time.Now().Add(time.Minute)) {
		t.Errorf("deadline = %v, %v; want one minute from the request", at, ok)
	}

	parent, cancel := context.WithDeadline(t.Context(), start.Add(time.Second))
	defer cancel()
	if at, ok := deadlineSeen(parent, time.Minute); !ok || !at.Equal(start.Add(time.Second)) {
		t.Errorf("deadline under an earlier parent = %v, %v; want the parent's", at, ok)
	}

	for _, d := range []time.Duration{0, -time.Second} {
		if at, ok := deadlineSeen(t.Context(), d); ok {
			t.Errorf("New(%v): deadline = %v, want none", d, at)
		}
	}
}

// TestNewTimeoutBecomes504 pins the cooperative contract: a handler that
// honours its context fails once the deadline passes and the error renders as
// 504, whether it returns the context error or one of its own.
func TestNewTimeoutBecomes504(t *testing.T) {
	t.Parallel()

	errDriver := errors.New("driver: query interrupted")
	for name, fail := range map[string]func(context.Context) error{
		"context error": func(ctx context.Context) error { return ctx.Err() },
		"own error":     func(context.Context) error { return errDriver },
	} {
		err := httpxmock.Run(request(t.Context()), func(c httpx.Context) error {
			<-c.Context().Done()
			return fail(c.Context())
		}, New(time.Millisecond))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("%s: err = %v, want context.DeadlineExceeded in its chain", name, err)
		}
		if got := httpz.ErrorStatus(err); got != http.StatusGatewayTimeout {
			t.Errorf("%s: status = %d, want 504", name, got)
		}
	}
	// The handler's own error stays reachable through the wrap.
	err := httpxmock.Run(request(t.Context()), func(c httpx.Context) error {
		<-c.Context().Done()
		return errDriver
	}, New(time.Millisecond))
	if !errors.Is(err, errDriver) {
		t.Errorf("err = %v, want the handler's error in its chain", err)
	}
}

// TestNewLeavesOtherOutcomesAlone pins what the middleware must not touch: an
// explicit status keeps deciding the response after the deadline, an error
// returned in time is passed through, and a handler that ignores its context
// succeeds.
func TestNewLeavesOtherOutcomesAlone(t *testing.T) {
	t.Parallel()

	err := httpxmock.Run(request(t.Context()), func(c httpx.Context) error {
		<-c.Context().Done()
		return httpx.NewNotFoundError("gone")
	}, New(time.Millisecond))
	if got := httpz.ErrorStatus(err); got != http.StatusNotFound {
		t.Errorf("explicit status after the deadline: status = %d, want 404", got)
	}

	errEarly := errors.New("early failure")
	err = httpxmock.Run(request(t.Context()), func(httpx.Context) error { return errEarly }, New(time.Minute))
	if err != errEarly {
		t.Errorf("error before the deadline = %v, want it unchanged", err)
	}

	err = httpxmock.Run(request(t.Context()), func(c httpx.Context) error {
		<-c.Context().Done() // the deadline passes, and the handler carries on
		return nil
	}, New(time.Millisecond))
	if err != nil {
		t.Errorf("handler ignoring the deadline: err = %v, want nil", err)
	}
}
