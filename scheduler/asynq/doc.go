// Package asynq is a scheduler.Scheduler on hibiken/asynq over a
// caller-owned *redis.Client.
//
// [NewScheduler] requires [WithClient]; the Redis client is never closed
// here. [Config.Queues] must include "default", where periodic tasks are
// enqueued. Register stores cron jobs under the task kind
// "scheduler.cron:<name>". Handle routes by exact task type, not asynq's mux
// prefix: "user.email" does not receive "user.email.welcome". Tasks with no
// bound handler return an error so asynq retries or archives them. Mux
// entries are never unmounted.
//
// # Usage
//
//	import (
//		"context"
//
//		"github.com/go-sphere/sphere/scheduler/asynq"
//	)
//
//	s, err := asynq.NewScheduler(asynq.Config{}, asynq.WithClient(rdb))
//	if err != nil {
//		return err
//	}
//	if err := s.Handle("email.welcome", func(ctx context.Context, payload []byte) error {
//		return nil
//	}); err != nil {
//		return err
//	}
//	// Run s as a task (task.Group or boot.Run). Producers may Enqueue:
//	id, err := s.Enqueue(ctx, "email.welcome", []byte("hi"))
//
// # Shutdown
//
// The Scheduler implements task.Task. Start blocks until Stop. A Stop that
// arrives before Start is honoured: Start then returns nil without starting
// the runtime. Stop returning ctx.Err() does not mean asynq is idle: wait for
// Stop or Close to return nil before closing the Redis client, because asynq
// keeps using it while it drains.
package asynq
