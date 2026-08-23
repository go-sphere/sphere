package boot_test

import (
	"context"
	"time"

	"github.com/go-sphere/sphere/core/boot"
	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/scripttask"
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

// One-shot recipe: the job runs, the group stops it, Run returns.
func ExampleRun_oneShot() {
	type conf struct{}
	_ = boot.Run(&conf{}, func(*conf) (*boot.Application, error) {
		job := scripttask.NewScriptTask("migrate", func(context.Context) error {
			return nil
		}, nil)
		return boot.NewApplication(job), nil
	})
}

// HTTP and companions start concurrently. Close Wire-owned clients after
// every Task.Stop, not as a sibling Task of the server.
func ExampleRun_httpAndInfra() {
	httpSrv := scripttask.NewScriptTask("http", nil, nil)
	consumer := scripttask.NewScriptTask("mq", nil, nil)
	_ = boot.NewApplication(httpSrv, consumer)
}

// Wire injectors often return (*App, cleanup, error). Run's builder cannot
// take the cleanup, so call it after Run returns — after every Task.Stop.
func ExampleRun_wireCleanup() {
	type conf struct{}
	app, cleanup, err := initializeExampleApp()
	if err != nil {
		return
	}
	defer cleanup()
	_ = boot.Run(&conf{}, func(*conf) (*boot.Application, error) {
		return app, nil
	})
}

// Ordered drain: last stage (HTTP) stops before the previous stage.
func ExampleRun_staged() {
	closer := scripttask.NewScriptTask("cache-trim", func(context.Context) error {
		return nil
	}, nil)
	httpSrv := scripttask.NewScriptTask("http", nil, nil)

	_ = boot.NewStagedApplication(
		[]task.Task{closer},
		[]task.Task{httpSrv},
	)
}

func ExampleNewApplicationFromGroup() {
	group := task.NewGroupWithOptions(nil, task.WithCleanupTimeout(5*time.Second))
	_ = boot.NewApplicationFromGroup(group)
}

func initializeExampleApp() (*boot.Application, func(), error) {
	return boot.NewApplication(), func() {}, nil
}
