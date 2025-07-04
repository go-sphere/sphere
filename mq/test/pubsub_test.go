package test

import (
	"context"
	"sync"
	"testing"

	"github.com/TBXark/sphere/mq"
	"github.com/TBXark/sphere/mq/memory"
	redismq "github.com/TBXark/sphere/mq/redis"
	"github.com/TBXark/sphere/server/conn/redis"
)

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
	wg.Wait()

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
	wg.Wait()
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
	client := redis.NewClient(&redis.Config{
		Addr: "localhost:6379",
		DB:   0,
	})
	pub, err := redismq.NewPubSub[int](
		redismq.WithClient(client),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	_, err = client.Ping(context.Background()).Result()
	if err != nil {
		t.Skipf("Redis server not available, skipping test: %v", err)
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
