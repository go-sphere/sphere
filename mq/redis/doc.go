// Package redis is the Redis-backed mq.MessageQueue: lists for Queue
// (RPUSH/BLPOP/LPOP) and Redis pub/sub for PubSub.
//
// Entry points: [NewQueue], [NewPubSub], or [NewMessageQueue] for both.
// [WithClient] is required and the client is never closed here; the caller
// closes it after the PubSub has stopped. The default codec is JSON
// ([WithCodec] replaces it).
//
// # Usage
//
//	import (
//		"github.com/go-sphere/sphere/mq/redis"
//		goredis "github.com/redis/go-redis/v9"
//	)
//
//	rdb := goredis.NewClient(&goredis.Options{Addr: addr})
//	defer rdb.Close()
//	q, err := redis.NewMessageQueue[Job](redis.WithClient(rdb))
//	if err != nil {
//		return err
//	}
//	defer q.Stop(ctx)
//	if err := q.Publish(ctx, "jobs", job); err != nil {
//		return err
//	}
//	got, err := q.Consume(ctx, "jobs")
//
// # Behavior
//
// Queue Close is a no-op. Consume cancellation is observed within about 1s
// (BLPOP poll). Decode failures after a pop return [DecodeError] with the raw
// bytes; TryConsume still reports found=true, so check err first. PubSub
// RequestStop cancels subscriptions but never closes the injected client;
// Broadcast and Subscribe then return mq.ErrPubSubClosed. Topic names are
// Redis keys: they collide with cache keys on the same DB. PubSub and
// MessageQueue implement task.Task; use [WithIdentifier] when a group
// contains more than one.
package redis
