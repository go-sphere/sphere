package boot_test

import (
	"context"
	"time"

	"github.com/go-sphere/sphere/core/boot"
	"github.com/go-sphere/sphere/infra/redis"
)

// ExampleAddBeforeStart_readinessProbe shows how to fail fast when a critical
// backend is unreachable at startup.
//
// Constructors such as redis.NewClient (and meilisearch.NewServiceManager) build
// their clients lazily and deliberately do NOT probe the backend at construction
// time, so a bad host does not block or panic inside New. When you want an eager
// readiness check, opt into one as a before-start hook. Use a bounded context so a
// hung dial cannot stall boot indefinitely; returning an error aborts startup.
func ExampleAddBeforeStart_readinessProbe() {
	client, err := redis.NewClient(redis.Config{URL: "redis://localhost:6379/0"})
	if err != nil {
		panic(err)
	}

	options := []boot.Option{
		boot.AddBeforeStart(func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			return client.Ping(ctx).Err()
		}),
	}

	// Pass the options through to boot.Run(conf, builder, options...).
	_ = options
}
