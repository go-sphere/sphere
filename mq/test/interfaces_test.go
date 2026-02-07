package test

import (
	"testing"

	"github.com/go-sphere/sphere/mq"
	"github.com/go-sphere/sphere/mq/memory"
	redismq "github.com/go-sphere/sphere/mq/redis"
	"github.com/go-sphere/sphere/test/redistest"
)

var (
	_ mq.Queue[int]        = (*memory.Queue[int])(nil)
	_ mq.PubSub[int]       = (*memory.PubSub[int])(nil)
	_ mq.MessageQueue[int] = (*memory.MessageQueue[int])(nil)
	_ mq.Queue[int]        = (*redismq.Queue[int])(nil)
	_ mq.PubSub[int]       = (*redismq.PubSub[int])(nil)
	_ mq.MessageQueue[int] = (*redismq.MessageQueue[int])(nil)
)

func TestRedisConstructorsValidation(t *testing.T) {
	t.Parallel()

	if _, err := redismq.NewQueue[int](); err == nil {
		t.Fatalf("expected NewQueue without client to fail")
	}
	if _, err := redismq.NewPubSub[int](); err == nil {
		t.Fatalf("expected NewPubSub without client to fail")
	}
	if _, err := redismq.NewMessageQueue[int](); err == nil {
		t.Fatalf("expected NewMessageQueue without client to fail")
	}
}

func TestMessageQueueConstruction(t *testing.T) {
	t.Parallel()

	m := memory.NewMessageQueue[int]()
	if m.Queue == nil || m.PubSub == nil {
		t.Fatalf("memory message queue components should not be nil")
	}
	if err := m.Close(); err != nil {
		t.Fatalf("memory message queue close: %v", err)
	}

	client := redistest.NewTestRedisClient(t)
	r, err := redismq.NewMessageQueue[int](redismq.WithClient(client))
	if err != nil {
		t.Fatalf("create redis message queue: %v", err)
	}
	if r.Queue == nil || r.PubSub == nil {
		t.Fatalf("redis message queue components should not be nil")
	}
	if err := r.Close(); err != nil {
		t.Fatalf("redis message queue close: %v", err)
	}
}
