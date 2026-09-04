package test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/sphere/mq"
)

func TestPubSubContract(t *testing.T) {
	t.Parallel()

	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			p := factory.newInt(t)

			const topic = "numbers"
			recv := make(chan int, 3)
			if _, err := p.Subscribe(ctx, topic, func(_ context.Context, data int) error {
				recv <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe: %v", err)
			}

			if err := p.Broadcast(ctx, topic, 1); err != nil {
				t.Fatalf("Broadcast first: %v", err)
			}
			if err := p.Broadcast(ctx, topic, 2); err != nil {
				t.Fatalf("Broadcast second: %v", err)
			}

			assertReceiveInt(t, recv, 1)
			assertReceiveInt(t, recv, 2)

			done, err := p.StopTopic(topic)
			if err != nil {
				t.Fatalf("StopTopic: %v", err)
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("StopTopic did not quiesce")
			}

			if err := p.Broadcast(ctx, topic, 3); err != nil {
				t.Fatalf("Broadcast after unsubscribe should not fail: %v", err)
			}
			assertNoReceiveInt(t, recv)
		})
	}
}

func TestPubSubMultiSubscribers(t *testing.T) {
	t.Parallel()

	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			p := factory.newInt(t)

			const topic = "fanout"
			recvA := make(chan int, 1)
			recvB := make(chan int, 1)

			if _, err := p.Subscribe(ctx, topic, func(_ context.Context, data int) error {
				recvA <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe A: %v", err)
			}
			if _, err := p.Subscribe(ctx, topic, func(_ context.Context, data int) error {
				recvB <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe B: %v", err)
			}

			if err := p.Broadcast(ctx, topic, 7); err != nil {
				t.Fatalf("Broadcast fanout: %v", err)
			}

			assertReceiveInt(t, recvA, 7)
			assertReceiveInt(t, recvB, 7)
		})
	}
}

func TestPubSubStopTopicMultipleSubscriptions(t *testing.T) {
	t.Parallel()

	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			p := factory.newInt(t)

			const topic = "unsubscribe-all"
			recvA := make(chan int, 2)
			recvB := make(chan int, 2)

			if _, err := p.Subscribe(ctx, topic, func(_ context.Context, data int) error {
				recvA <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe A: %v", err)
			}
			if _, err := p.Subscribe(ctx, topic, func(_ context.Context, data int) error {
				recvB <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe B: %v", err)
			}

			if err := p.Broadcast(ctx, topic, 1); err != nil {
				t.Fatalf("Broadcast before unsubscribe: %v", err)
			}
			assertReceiveInt(t, recvA, 1)
			assertReceiveInt(t, recvB, 1)

			done, err := p.StopTopic(topic)
			if err != nil {
				t.Fatalf("StopTopic: %v", err)
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("StopTopic did not quiesce")
			}
			if err := p.Broadcast(ctx, topic, 2); err != nil {
				t.Fatalf("Broadcast after unsubscribe: %v", err)
			}
			assertNoReceiveInt(t, recvA)
			assertNoReceiveInt(t, recvB)
		})
	}
}

func TestPubSubStructPayload(t *testing.T) {
	t.Parallel()

	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			p := factory.newPayload(t)

			want := payload{
				ID:    42,
				Name:  "sphere",
				Meta:  map[string]string{"env": "test"},
				Flags: []bool{true, false, true},
			}

			recv := make(chan payload, 1)
			if _, err := p.Subscribe(ctx, "struct-topic", func(_ context.Context, data payload) error {
				recv <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe struct payload: %v", err)
			}

			if err := p.Broadcast(ctx, "struct-topic", want); err != nil {
				t.Fatalf("Broadcast struct payload: %v", err)
			}

			got := assertReceivePayload(t, recv)
			if got.ID != want.ID || got.Name != want.Name || got.Meta["env"] != want.Meta["env"] {
				t.Fatalf("payload mismatch: got=%+v want=%+v", got, want)
			}
			if len(got.Flags) != len(want.Flags) {
				t.Fatalf("flags length mismatch: got=%d want=%d", len(got.Flags), len(want.Flags))
			}
			for i := range want.Flags {
				if got.Flags[i] != want.Flags[i] {
					t.Fatalf("flag mismatch at %d: got=%v want=%v", i, got.Flags[i], want.Flags[i])
				}
			}
		})
	}
}

func TestPubSubContinuesAfterHandlerPanic(t *testing.T) {
	t.Parallel()

	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			p := factory.newInt(t)
			received := make(chan int, 1)

			if _, err := p.Subscribe(ctx, "panic-recovery", func(_ context.Context, data int) error {
				if data == 1 {
					panic("handler panic")
				}
				received <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe: %v", err)
			}
			if err := p.Broadcast(ctx, "panic-recovery", 1); err != nil {
				t.Fatalf("Broadcast panic message: %v", err)
			}
			if err := p.Broadcast(ctx, "panic-recovery", 2); err != nil {
				t.Fatalf("Broadcast following message: %v", err)
			}

			assertReceiveInt(t, received, 2)
		})
	}
}

func TestPubSubClose(t *testing.T) {
	t.Parallel()

	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Parallel()

			p := factory.newInt(t)
			if err := p.Stop(context.Background()); err != nil {
				t.Fatalf("Stop: %v", err)
			}
			if err := p.Stop(context.Background()); err != nil {
				t.Fatalf("Stop idempotent check: %v", err)
			}
		})
	}
}

func assertReceiveInt(t *testing.T, ch <-chan int, want int) {
	t.Helper()

	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("received value mismatch: got=%d want=%d", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for value=%d", want)
	}
}

func assertNoReceiveInt(t *testing.T, ch <-chan int) {
	t.Helper()

	select {
	case got := <-ch:
		t.Fatalf("unexpected message after unsubscribe: got=%d", got)
	case <-time.After(150 * time.Millisecond):
	}
}

func assertReceivePayload(t *testing.T, ch <-chan payload) payload {
	t.Helper()

	select {
	case got := <-ch:
		return got
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for payload")
		return payload{}
	}
}

// TestPubSubStopWaitsForRunningHandler pins that task.Task.Stop is a real
// quiesce point.
//
// Ordered shutdown depends on it: a task group closes the pubsub and then goes
// on to close the database and the log backend, so a Close that returns while a
// handler is still running pulls those out from under it. The handler has no
// other way to signal that it is finished, and the scheduler package already
// commits to the same rule, so the two subsystems must not disagree.
func TestPubSubStopWaitsForRunningHandler(t *testing.T) {
	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ctx := t.Context()
			ps := factory.newInt(t)

			entered := make(chan struct{})
			release := make(chan struct{})
			if _, err := ps.Subscribe(ctx, "topic", func(context.Context, int) error {
				close(entered)
				<-release
				return nil
			}); err != nil {
				t.Fatalf("Subscribe: %v", err)
			}

			if err := ps.Broadcast(ctx, "topic", 1); err != nil {
				t.Fatalf("Broadcast: %v", err)
			}

			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("handler never ran")
			}

			stopResult := make(chan error, 1)
			go func() { stopResult <- ps.Stop(t.Context()) }()
			select {
			case err := <-stopResult:
				t.Fatalf("Stop returned before handler finished: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			close(release)
			if err := <-stopResult; err != nil {
				t.Fatalf("Stop: %v", err)
			}
		})
	}
}

func TestPubSubHandlerCanStopPubSub(t *testing.T) {
	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ps := factory.newInt(t)
			stopResult := make(chan error, 1)
			if _, err := ps.Subscribe(t.Context(), "topic", func(context.Context, int) error {
				stopResult <- ps.RequestStop()
				return nil
			}); err != nil {
				t.Fatalf("Subscribe: %v", err)
			}
			if err := ps.Broadcast(t.Context(), "topic", 1); err != nil {
				t.Fatalf("Broadcast: %v", err)
			}

			select {
			case err := <-stopResult:
				if err != nil {
					t.Fatalf("RequestStop from handler: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("RequestStop called from handler deadlocked")
			}
			select {
			case <-ps.Done():
			case <-time.After(2 * time.Second):
				t.Fatal("Done did not close after handler returned")
			}
			if err := ps.Broadcast(t.Context(), "topic", 2); !errors.Is(err, mq.ErrPubSubClosed) {
				t.Fatalf("Broadcast after Stop error = %v, want ErrPubSubClosed", err)
			}
			if _, err := ps.Subscribe(t.Context(), "topic", func(context.Context, int) error { return nil }); !errors.Is(err, mq.ErrPubSubClosed) {
				t.Fatalf("Subscribe after Stop error = %v, want ErrPubSubClosed", err)
			}
		})
	}
}

func TestPubSubHandlerCanStopTopic(t *testing.T) {
	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ps := factory.newInt(t)
			topicDone := make(chan (<-chan struct{}), 1)
			stopErr := make(chan error, 1)
			if _, err := ps.Subscribe(t.Context(), "topic", func(context.Context, int) error {
				done, err := ps.StopTopic("topic")
				stopErr <- err
				topicDone <- done
				return nil
			}); err != nil {
				t.Fatalf("Subscribe: %v", err)
			}
			if err := ps.Broadcast(t.Context(), "topic", 1); err != nil {
				t.Fatalf("Broadcast: %v", err)
			}

			select {
			case err := <-stopErr:
				if err != nil {
					t.Fatalf("StopTopic from handler: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("StopTopic called from handler deadlocked")
			}
			var done <-chan struct{}
			select {
			case done = <-topicDone:
			case <-time.After(2 * time.Second):
				t.Fatal("StopTopic did not return a completion channel")
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("topic did not quiesce after handler returned")
			}

			received := make(chan int, 1)
			if _, err := ps.Subscribe(t.Context(), "topic", func(_ context.Context, data int) error {
				received <- data
				return nil
			}); err != nil {
				t.Fatalf("Subscribe new topic generation: %v", err)
			}
			if err := ps.Broadcast(t.Context(), "topic", 2); err != nil {
				t.Fatalf("Broadcast to new topic generation: %v", err)
			}
			assertReceiveInt(t, received, 2)
		})
	}
}

func TestPubSubSubscriptionContextOwnsLifetime(t *testing.T) {
	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ps := factory.newInt(t)
			subCtx, cancel := context.WithCancel(context.Background())
			entered := make(chan struct{})
			cancelled := make(chan error, 1)
			sub, err := ps.Subscribe(subCtx, "topic", func(ctx context.Context, _ int) error {
				close(entered)
				<-ctx.Done()
				cancelled <- ctx.Err()
				return nil
			})
			if err != nil {
				t.Fatalf("Subscribe: %v", err)
			}
			if err := ps.Broadcast(t.Context(), "topic", 1); err != nil {
				t.Fatalf("Broadcast: %v", err)
			}
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("handler did not start")
			}
			cancel()
			select {
			case err := <-cancelled:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("handler context error = %v, want context.Canceled", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("handler did not observe subscription cancellation")
			}
			select {
			case <-sub.Done():
			case <-time.After(2 * time.Second):
				t.Fatal("subscription did not stop after context cancellation")
			}
		})
	}
}

func TestPubSubStopHonorsContext(t *testing.T) {
	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ps := factory.newInt(t)
			entered := make(chan struct{})
			release := make(chan struct{})
			if _, err := ps.Subscribe(t.Context(), "topic", func(context.Context, int) error {
				close(entered)
				<-release
				return nil
			}); err != nil {
				t.Fatalf("Subscribe: %v", err)
			}
			if err := ps.Broadcast(t.Context(), "topic", 1); err != nil {
				t.Fatalf("Broadcast: %v", err)
			}
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("handler did not start")
			}

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			if err := ps.Stop(shutdownCtx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Stop error = %v, want context.DeadlineExceeded", err)
			}
			close(release)
			select {
			case <-ps.Done():
			case <-time.After(2 * time.Second):
				t.Fatal("Done did not close after blocked handler returned")
			}
		})
	}
}

func TestPubSubStopDropsBufferedMessages(t *testing.T) {
	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ps := factory.newInt(t)
			entered := make(chan struct{})
			release := make(chan struct{})
			var calls atomic.Int32
			if _, err := ps.Subscribe(t.Context(), "topic", func(context.Context, int) error {
				if calls.Add(1) == 1 {
					close(entered)
					<-release
				}
				return nil
			}); err != nil {
				t.Fatalf("Subscribe: %v", err)
			}
			if err := ps.Broadcast(t.Context(), "topic", 1); err != nil {
				t.Fatalf("Broadcast first: %v", err)
			}
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("first handler did not start")
			}
			if err := ps.Broadcast(t.Context(), "topic", 2); err != nil {
				t.Fatalf("Broadcast buffered: %v", err)
			}
			if err := ps.RequestStop(); err != nil {
				t.Fatalf("RequestStop: %v", err)
			}
			close(release)
			select {
			case <-ps.Done():
			case <-time.After(2 * time.Second):
				t.Fatal("Done did not close")
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("handler calls after Stop = %d, want 1", got)
			}
		})
	}
}

func TestPubSubStopTopicWaitsOnlyForThatTopic(t *testing.T) {
	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ps := factory.newInt(t)
			enteredA := make(chan struct{})
			enteredB := make(chan struct{})
			releaseA := make(chan struct{})
			releaseB := make(chan struct{})
			var releaseAOnce sync.Once
			var releaseBOnce sync.Once
			defer releaseAOnce.Do(func() { close(releaseA) })
			defer releaseBOnce.Do(func() { close(releaseB) })

			if _, err := ps.Subscribe(t.Context(), "a", func(context.Context, int) error {
				close(enteredA)
				<-releaseA
				return nil
			}); err != nil {
				t.Fatalf("Subscribe a: %v", err)
			}
			if _, err := ps.Subscribe(t.Context(), "b", func(context.Context, int) error {
				close(enteredB)
				<-releaseB
				return nil
			}); err != nil {
				t.Fatalf("Subscribe b: %v", err)
			}
			if err := ps.Broadcast(t.Context(), "a", 1); err != nil {
				t.Fatalf("Broadcast a: %v", err)
			}
			if err := ps.Broadcast(t.Context(), "b", 1); err != nil {
				t.Fatalf("Broadcast b: %v", err)
			}
			select {
			case <-enteredA:
			case <-time.After(2 * time.Second):
				t.Fatal("topic a handler did not start")
			}
			select {
			case <-enteredB:
			case <-time.After(2 * time.Second):
				t.Fatal("topic b handler did not start")
			}

			done, err := ps.StopTopic("a")
			if err != nil {
				t.Fatalf("StopTopic a: %v", err)
			}
			select {
			case <-done:
				t.Fatal("topic a stopped before its running handler returned")
			case <-time.After(50 * time.Millisecond):
			}
			releaseAOnce.Do(func() { close(releaseA) })
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("topic a waited for unrelated topic b")
			}
			releaseBOnce.Do(func() { close(releaseB) })
		})
	}
}

func TestPubSubTaskLifecycle(t *testing.T) {
	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ps := factory.newInt(t)
			if ps.Identifier() == "" {
				t.Fatal("Identifier must not be empty")
			}

			startResult := make(chan error, 1)
			go func() {
				startResult <- ps.Start(context.Background())
			}()
			select {
			case err := <-startResult:
				t.Fatalf("Start returned before Stop: %v", err)
			case <-time.After(50 * time.Millisecond):
			}

			if err := ps.Stop(context.Background()); err != nil {
				t.Fatalf("Stop: %v", err)
			}
			select {
			case err := <-startResult:
				if err != nil {
					t.Fatalf("Start after Stop: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Stop did not unblock Start")
			}

			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			if err := ps.Start(cancelled); err != nil {
				t.Fatalf("Start after completed Stop: %v", err)
			}
			if err := ps.Stop(cancelled); err != nil {
				t.Fatalf("idempotent Stop after completion: %v", err)
			}
		})
	}
}

func TestPubSubStartHonorsContext(t *testing.T) {
	for _, factory := range pubSubFactories() {
		t.Run(factory.name, func(t *testing.T) {
			ps := factory.newInt(t)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := ps.Start(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("Start error = %v, want context.Canceled", err)
			}
			if err := ps.Stop(context.Background()); err != nil {
				t.Fatalf("Stop after cancelled Start: %v", err)
			}
		})
	}
}
