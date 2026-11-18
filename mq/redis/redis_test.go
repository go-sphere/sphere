package redis

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
	"github.com/go-sphere/sphere/internal/redistest"
	"github.com/go-sphere/sphere/mq"
)

func TestOptionsValidationAndNilOption(t *testing.T) {
	client := redistest.NewTestRedisClient(t)

	if _, err := NewQueue[int](); err == nil {
		t.Fatal("NewQueue without client succeeded, want error")
	}
	if _, err := NewPubSub[int](WithClient(client), WithCodec(nil)); err == nil {
		t.Fatal("NewPubSub with nil codec succeeded, want error")
	}
	if _, err := NewMessageQueue[int](WithClient(client), nil, WithIdentifier("custom-mq")); err != nil {
		t.Fatalf("NewMessageQueue with valid options failed: %v", err)
	}
	mqInst, err := NewMessageQueue[int](WithClient(client), WithIdentifier("custom-mq"), WithIdentifier(""))
	if err != nil {
		t.Fatalf("NewMessageQueue: %v", err)
	}
	if got := mqInst.Identifier(); got != "custom-mq" {
		t.Fatalf("Identifier() = %q, want custom-mq", got)
	}
}

func TestDecodeErrorMethods(t *testing.T) {
	inner := errors.New("bad json")
	err := &DecodeError{
		Topic: "orders",
		Raw:   []byte("{"),
		Err:   inner,
	}
	if !errors.Is(err, inner) {
		t.Fatalf("errors.Is(DecodeError, inner) = false")
	}
	if !strings.Contains(err.Error(), "orders") {
		t.Fatalf("DecodeError.Error() = %q, want topic name", err.Error())
	}
}

func TestQueuePublishCanceledContextSkipsMarshal(t *testing.T) {
	client := redistest.NewTestRedisClient(t)
	marshals := 0
	spyCodec := codec.NewCodec(
		func(any) ([]byte, error) {
			marshals++
			return []byte("1"), nil
		},
		func([]byte, any) error { return nil },
	)
	q, err := NewQueue[int](WithClient(client), WithCodec(spyCodec))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := q.Publish(ctx, "t", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish(canceled) = %v, want context.Canceled", err)
	}
	if marshals != 0 {
		t.Fatalf("marshal calls = %d, want 0", marshals)
	}
}

func TestPubSubSubscriptionStopWaitsAndCloseLeavesClientOpen(t *testing.T) {
	client := redistest.NewTestRedisClient(t)
	ps, err := NewPubSub[int](WithClient(client))
	if err != nil {
		t.Fatalf("NewPubSub: %v", err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	releaseOnce := sync.OnceFunc(func() { close(release) })
	defer releaseOnce()
	once := sync.OnceFunc(func() { close(entered) })
	sub, err := ps.Subscribe(t.Context(), "topic", func(context.Context, int) error {
		once()
		<-release
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
		t.Fatal("handler did not enter")
	}

	if err := sub.Stop(); err != nil {
		t.Fatalf("sub.Stop(): %v", err)
	}
	done, err := ps.StopTopic("topic")
	if err != nil {
		t.Fatalf("StopTopic(): %v", err)
	}
	select {
	case <-done:
		t.Fatal("StopTopic closed done before stopped subscription handler finished")
	case <-time.After(50 * time.Millisecond):
	}
	releaseOnce()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StopTopic did not close after handler finished")
	}

	if err := ps.Stop(t.Context()); err != nil {
		t.Fatalf("ps.Stop: %v", err)
	}
	if err := ps.Broadcast(t.Context(), "topic", 2); !errors.Is(err, mq.ErrPubSubClosed) {
		t.Fatalf("Broadcast after Stop = %v, want mq.ErrPubSubClosed", err)
	}
	if err := client.Ping(t.Context()).Err(); err != nil {
		t.Fatalf("injected redis client closed unexpectedly: %v", err)
	}
}

func TestQueueDecodeErrorOnConsumeAndTryConsume(t *testing.T) {
	ctx := t.Context()
	client := redistest.NewTestRedisClient(t)
	q, err := NewQueue[int](WithClient(client))
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}

	badPayload := []byte("not-an-int")
	if err := client.RPush(ctx, "decode-try", badPayload).Err(); err != nil {
		t.Fatalf("RPush: %v", err)
	}
	_, found, err := q.TryConsume(ctx, "decode-try")
	var decErr *DecodeError
	if !errors.As(err, &decErr) || !found || !bytes.Equal(decErr.Raw, badPayload) {
		t.Fatalf("TryConsume undecodable = (found=%v, err=%v), want found=true & *DecodeError", found, err)
	}

	if err := client.RPush(ctx, "decode-consume", badPayload).Err(); err != nil {
		t.Fatalf("RPush consume: %v", err)
	}
	_, err = q.Consume(ctx, "decode-consume")
	decErr = nil
	if !errors.As(err, &decErr) || !bytes.Equal(decErr.Raw, badPayload) {
		t.Fatalf("Consume undecodable = %v, want *DecodeError with raw payload", err)
	}
}

func TestPubSubCustomCodecPanicRecoveryAndMessageQueueLifecycle(t *testing.T) {
	client := redistest.NewTestRedisClient(t)
	panickingCodec := codec.NewCodec(
		codec.JsonCodec().Marshal,
		func(data []byte, v any) error {
			if string(data) == `"boom"` {
				panic("codec boom")
			}
			return codec.JsonCodec().Unmarshal(data, v)
		},
	)
	ps, err := NewPubSub[string](WithClient(client), WithCodec(panickingCodec))
	if err != nil {
		t.Fatalf("NewPubSub: %v", err)
	}
	t.Cleanup(func() { _ = ps.Stop(context.Background()) })

	received := make(chan string, 1)
	if _, err := ps.Subscribe(t.Context(), "codec-panic", func(_ context.Context, msg string) error {
		received <- msg
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := ps.Broadcast(t.Context(), "codec-panic", "boom"); err != nil {
		t.Fatalf("Broadcast boom: %v", err)
	}
	if err := ps.Broadcast(t.Context(), "codec-panic", "ok"); err != nil {
		t.Fatalf("Broadcast ok: %v", err)
	}
	select {
	case got := <-received:
		if got != "ok" {
			t.Fatalf("received %q, want ok", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("subscription did not stay alive after codec panic")
	}

	tasktest.AssertLifecycleContract(t, func() task.Task {
		mqInst, err := NewMessageQueue[int](WithClient(client))
		if err != nil {
			t.Fatalf("NewMessageQueue: %v", err)
		}
		return mqInst
	})
}
