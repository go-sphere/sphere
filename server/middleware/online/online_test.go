package online

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
)

// Online must stay a task.Task: its storage is only bounded while the periodic
// sweep is running, because the backing cache reclaims an expired entry when
// that key is read again or the map is swept, and the middleware only ever
// writes. Dropping the interface would silently reintroduce unbounded growth
// for a high-cardinality key such as a client IP.
var _ task.Task = (*Online)(nil)

func TestOnlineLifecycleContract(t *testing.T) {
	tasktest.AssertLifecycleContract(t, func() task.Task {
		return NewOnline(WithTrimInterval(10 * time.Millisecond))
	})
}

// TestSweepReclaimsExpiredEntries checks that entries do not survive their TTL
// once the tracker is running.
//
// It observes through OnlineCount, which reclaims as a side effect, so it
// cannot distinguish "the sweep reclaimed it" from "this call did". What it
// pins is the externally promised behaviour — an expired entry stops being
// counted — and, with the interface assertion above, that the sweep has
// somewhere to run.
func TestSweepReclaimsExpiredEntries(t *testing.T) {
	o := NewOnline(WithTrimInterval(10 * time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = o.Start(ctx) }()
	t.Cleanup(func() { _ = o.Stop(context.Background()) })

	for _, key := range []string{"ip-1", "ip-2", "ip-3"} {
		if err := o.cache.SetWithTTL(ctx, key, struct{}{}, 20*time.Millisecond); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	if got := o.OnlineCount(); got != 3 {
		t.Fatalf("OnlineCount() = %d, want 3", got)
	}

	deadline := time.After(2 * time.Second)
	for {
		if o.OnlineCount() == 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expired entries were still counted: %d", o.OnlineCount())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// TestZeroValueStartErrors covers an Online built via its zero value rather
// than NewOnline. Start must fail with ErrNotInitialized instead of panicking
// in time.NewTicker on the zero trim interval.
func TestZeroValueStartErrors(t *testing.T) {
	var o Online
	if err := o.Start(context.Background()); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Start on zero-value Online: got %v, want ErrNotInitialized", err)
	}
}

// TestWithTrimIntervalIgnoresNonPositive pins that a bad interval keeps the
// default instead of panicking in time.NewTicker on Start.
func TestWithTrimIntervalIgnoresNonPositive(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		if got := NewOnline(WithTrimInterval(interval)).trimInterval; got != defaultTrimInterval {
			t.Errorf("WithTrimInterval(%v) = %v, want the default %v", interval, got, defaultTrimInterval)
		}
	}
}
