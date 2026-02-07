package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-sphere/sphere/mq"
	"github.com/go-sphere/sphere/mq/memory"
	redismq "github.com/go-sphere/sphere/mq/redis"
	"github.com/go-sphere/sphere/test/redistest"
)

func testQueue(t *testing.T, queue mq.Queue[int], testBlocking bool) {
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

	_, found, err := queue.TryConsume(ctx, topic2)
	if err != nil {
		t.Fatalf("expected no error on empty non-blocking consume, got %v", err)
	}
	if found {
		t.Fatal("expected no message on empty non-blocking consume")
	}

	if testBlocking {
		blockingTopic := "test-topic-blocking"
		go func() {
			time.Sleep(50 * time.Millisecond)
			_ = queue.Publish(context.Background(), blockingTopic, 9)
		}()
		blockingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		data, err = queue.Consume(blockingCtx, blockingTopic)
		if err != nil {
			t.Fatalf("expected no error on blocking consume, got %v", err)
		}
		if data != 9 {
			t.Fatalf("expected data to be 9, got %d", data)
		}

		emptyTopic := "test-empty-topic"
		nonBlockingCtx, nonBlockingCancel := context.WithTimeout(ctx, 120*time.Millisecond)
		defer nonBlockingCancel()
		_, err = queue.Consume(nonBlockingCtx, emptyTopic)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded for empty blocking consume, got %v", err)
		}
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
	testQueue(t, queue, true)
	t.Log("TestQueue_Memory passed")
}

func TestQueue_Redis(t *testing.T) {
	client := redistest.NewTestRedisClient(t)
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
	testQueue(t, queue, false)
	t.Log("TestQueue_Redis passed")
}
