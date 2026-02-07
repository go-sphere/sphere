package test

import (
	"testing"

	"github.com/go-sphere/sphere/mq"
	"github.com/go-sphere/sphere/mq/memory"
	redismq "github.com/go-sphere/sphere/mq/redis"
	"github.com/go-sphere/sphere/test/redistest"
)

type queueFactory struct {
	name                 string
	blockingConsumeCheck bool
	new                  func(tb testing.TB) mq.Queue[int]
}

type pubSubFactory struct {
	name       string
	newInt     func(tb testing.TB) mq.PubSub[int]
	newPayload func(tb testing.TB) mq.PubSub[payload]
}

func queueFactories() []queueFactory {
	return []queueFactory{
		{
			name:                 "memory",
			blockingConsumeCheck: true,
			new: func(tb testing.TB) mq.Queue[int] {
				tb.Helper()
				q := memory.NewQueue[int]()
				tb.Cleanup(func() { _ = q.Close() })
				return q
			},
		},
		{
			name:                 "redis",
			blockingConsumeCheck: false,
			new: func(tb testing.TB) mq.Queue[int] {
				t, ok := tb.(*testing.T)
				if !ok {
					tb.Fatalf("redis queue factory requires *testing.T")
				}
				client := redistest.NewTestRedisClient(t)
				q, err := redismq.NewQueue[int](redismq.WithClient(client))
				if err != nil {
					tb.Fatalf("create redis queue: %v", err)
				}
				tb.Cleanup(func() { _ = q.Close() })
				return q
			},
		},
	}
}

func pubSubFactories() []pubSubFactory {
	return []pubSubFactory{
		{
			name: "memory",
			newInt: func(tb testing.TB) mq.PubSub[int] {
				tb.Helper()
				p := memory.NewPubSub[int]()
				tb.Cleanup(func() { _ = p.Close() })
				return p
			},
			newPayload: func(tb testing.TB) mq.PubSub[payload] {
				tb.Helper()
				p := memory.NewPubSub[payload]()
				tb.Cleanup(func() { _ = p.Close() })
				return p
			},
		},
		{
			name: "redis",
			newInt: func(tb testing.TB) mq.PubSub[int] {
				t, ok := tb.(*testing.T)
				if !ok {
					tb.Fatalf("redis pubsub factory requires *testing.T")
				}
				client := redistest.NewTestRedisClient(t)
				p, err := redismq.NewPubSub[int](redismq.WithClient(client))
				if err != nil {
					tb.Fatalf("create redis pubsub[int]: %v", err)
				}
				tb.Cleanup(func() { _ = p.Close() })
				return p
			},
			newPayload: func(tb testing.TB) mq.PubSub[payload] {
				t, ok := tb.(*testing.T)
				if !ok {
					tb.Fatalf("redis pubsub factory requires *testing.T")
				}
				client := redistest.NewTestRedisClient(t)
				p, err := redismq.NewPubSub[payload](redismq.WithClient(client))
				if err != nil {
					tb.Fatalf("create redis pubsub[payload]: %v", err)
				}
				tb.Cleanup(func() { _ = p.Close() })
				return p
			},
		},
	}
}

type payload struct {
	ID    int               `json:"id"`
	Name  string            `json:"name"`
	Meta  map[string]string `json:"meta"`
	Flags []bool            `json:"flags"`
}
