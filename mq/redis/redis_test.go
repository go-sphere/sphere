package redis

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/sphere/mq"
	"github.com/go-sphere/sphere/test/redistest"
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
	close(release)
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
