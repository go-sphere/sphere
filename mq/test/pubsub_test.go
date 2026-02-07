package test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-sphere/sphere/mq"
	"github.com/go-sphere/sphere/mq/memory"
	redismq "github.com/go-sphere/sphere/mq/redis"
	"github.com/go-sphere/sphere/test/redistest"
)

func waitGroupWithTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration, reason string) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()

	select {
	case <-done:
		return
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %s after %s", reason, timeout)
	}
}

func testPubSub(t *testing.T, pub mq.PubSub[int]) {
	var wg sync.WaitGroup
	topic := "test-topic"
	ctx := context.Background()
	wg.Add(1)
	err := pub.Subscribe(ctx, topic, func(data int) error {
		defer wg.Done()
		if data != 1 {
			t.Fatalf("expected data to be 1, got %d", data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = pub.Broadcast(ctx, topic, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	waitGroupWithTimeout(t, &wg, 2*time.Second, "first pub/sub delivery")

	topic2 := "test-topic-2"
	wg.Add(2)
	err = pub.Subscribe(ctx, topic2, func(data int) error {
		defer wg.Done()
		if data != 2 && data != 3 {
			t.Fatalf("expected data to be 2 or 3, got %d", data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = pub.Broadcast(ctx, topic2, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = pub.Broadcast(ctx, topic2, 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	waitGroupWithTimeout(t, &wg, 2*time.Second, "second pub/sub delivery")
}

func TestPubSub_Memory(t *testing.T) {
	pub := memory.NewPubSub[int]()
	defer func() {
		err := pub.Close()
		if err != nil {
			t.Fatalf("expected no error on close, got %v", err)
		}
	}()
	testPubSub(t, pub)
	t.Log("TestPubSub_Memory passed")
}

func TestPubSub_Redis(t *testing.T) {
	client := redistest.NewTestRedisClient(t)
	pub, err := redismq.NewPubSub[int](
		redismq.WithClient(client),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer func() {
		qErr := pub.Close()
		if qErr != nil {
			t.Fatalf("expected no error on close, got %v", qErr)
		}
	}()
	testPubSub(t, pub)
	t.Log("TestPubSub_Redis passed")
}

type payload struct {
	ID    int               `json:"id"`
	Name  string            `json:"name"`
	Meta  map[string]string `json:"meta"`
	Flags []bool            `json:"flags"`
}

func testPubSubStruct(t *testing.T, pub mq.PubSub[payload]) {
	topic := "test-topic-struct"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	want := payload{
		ID:   42,
		Name: "sphere",
		Meta: map[string]string{"env": "test"},
		Flags: []bool{
			true,
			false,
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	err := pub.Subscribe(ctx, topic, func(data payload) error {
		defer wg.Done()
		if data.ID != want.ID || data.Name != want.Name {
			t.Fatalf("unexpected payload identity: got %+v", data)
		}
		if data.Meta["env"] != want.Meta["env"] {
			t.Fatalf("unexpected payload meta: got %+v", data.Meta)
		}
		if len(data.Flags) != len(want.Flags) {
			t.Fatalf("unexpected flags length: got %d want %d", len(data.Flags), len(want.Flags))
		}
		for i := range want.Flags {
			if data.Flags[i] != want.Flags[i] {
				t.Fatalf("unexpected flag at index %d: got %v want %v", i, data.Flags[i], want.Flags[i])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	err = pub.Broadcast(ctx, topic, want)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	waitGroupWithTimeout(t, &wg, 2*time.Second, "struct pub/sub delivery")
}

func TestPubSubStruct_Memory(t *testing.T) {
	pub := memory.NewPubSub[payload]()
	defer func() {
		err := pub.Close()
		if err != nil {
			t.Fatalf("expected no error on close, got %v", err)
		}
	}()
	testPubSubStruct(t, pub)
}

func TestPubSubStruct_Redis(t *testing.T) {
	client := redistest.NewTestRedisClient(t)
	pub, err := redismq.NewPubSub[payload](
		redismq.WithClient(client),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer func() {
		qErr := pub.Close()
		if qErr != nil {
			t.Fatalf("expected no error on close, got %v", qErr)
		}
	}()
	testPubSubStruct(t, pub)
}
