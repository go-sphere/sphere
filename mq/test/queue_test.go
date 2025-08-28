package test

import (
	"context"
	"testing"

	"github.com/go-sphere/sphere/infra/redis"
	"github.com/go-sphere/sphere/mq"
	"github.com/go-sphere/sphere/mq/memory"
	redismq "github.com/go-sphere/sphere/mq/redis"
)

func testQueue(t *testing.T, queue mq.Queue[int]) {
	topic := "test-topic"
	ctx := context.Background()
	err := queue.Publish(ctx, topic, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	data, err := queue.Consume(ctx, topic)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if data != 1 {
		t.Fatalf("expected data to be 1, got %d", data)
	}
	topic2 := "test-topic-2"
	err = queue.Publish(ctx, topic2, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = queue.Publish(ctx, topic2, 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	data, err = queue.Consume(ctx, topic2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if data != 2 {
		t.Fatalf("expected data to be 2, got %d", data)
	}
	data, err = queue.Consume(ctx, topic2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if data != 3 {
		t.Fatalf("expected data to be 3, got %d", data)
	}
}

func TestQueue_Memory(t *testing.T) {
	queue := memory.NewQueue[int]()
	defer func() {
		err := queue.Close()
		if err != nil {
			t.Fatalf("expected no error on close, got %v", err)
		}
	}()
	testQueue(t, queue)
	t.Log("TestQueue_Memory passed")
}

func TestQueue_Redis(t *testing.T) {
	client, err := redis.NewClient(&redis.Config{
		URL: "redis://localhost:6379/0",
	})
	if err != nil {
		t.Skipf("Redis server not available, skipping test: %v", err)
	}
	queue, err := redismq.NewQueue[int](
		redismq.WithClient(client),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer func() {
		qErr := queue.Close()
		if qErr != nil {
			t.Fatalf("expected no error on close, got %v", qErr)
		}
	}()
	testQueue(t, queue)
	t.Log("TestQueue_Redis passed")
}
