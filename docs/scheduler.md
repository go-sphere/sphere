# Scheduler

The `scheduler` package provides infrastructure interfaces for periodic jobs
and asynchronous task queues. Drivers expose only the capabilities they support:

- `scheduler.Cron` registers periodic jobs.
- `scheduler.Producer` enqueues asynchronous jobs.
- `scheduler.Consumer` registers asynchronous job handlers.
- `scheduler.Scheduler` combines all capabilities plus `io.Closer`.

Two drivers are available:

- `scheduler/cron` wraps `github.com/robfig/cron/v3` and supports periodic jobs.
- `scheduler/asynq` wraps `github.com/hibiken/asynq` and supports periodic jobs,
  producers, consumers, and lifecycle integration.

Both drivers implement `core/task.Task`, so they can be returned from a
`boot.Run` builder with other long-running services.

## Periodic jobs require a single instance

Neither driver coordinates periodic jobs across processes. Every replica
evaluates the schedule locally and fires on its own, so `N` replicas run each
periodic job `N` times per tick — the cron driver invokes the handler directly,
and the asynq driver enqueues one task per replica. asynq's scheduler is
documented as requiring a single running instance for exactly this reason, and
`Register` adds no deduplication key.

This matters because the natural way to use the package — returning the
scheduler from a `boot.Run` builder alongside an HTTP server, as in the example
below — ties it to a deployment unit that is normally scaled horizontally.
Either run the scheduler in its own single-replica deployment, or make the
handlers idempotent and guard them with an external lock. Asynchronous task
handlers (`Handle`/`Enqueue`) have no such restriction: the queue distributes
each task to exactly one consumer, so those replicas scale freely.

## Task kind routing

Handlers are routed by exact kind. A kind with no registered handler is not
delivered to a handler registered under a prefix of it, and is reported as a
failure so the driver's normal retry and archive behavior applies.

## Error Semantics

| Scenario | asynq driver | cron driver |
| --- | --- | --- |
| Handler returns error | Uses asynq retry and archive behavior according to enqueue options. | Logs the error once and does not retry. |
| Handler panics | Panic is recovered, logged, and returned as an error so asynq can apply retry policy. | Panic is recovered and logged by cron middleware; no retry is attempted. |
| Task kind has no handler | Returns an error, so asynq applies its normal retry and archive behavior. This also covers a kind whose handler was unregistered while the task was queued. | Not applicable. |
| Context timeout or cancel | The task fails according to asynq context/deadline handling and retry policy. | The current invocation observes the scheduler lifecycle context if the handler uses it. |
| Register or enqueue after close | Returns `scheduler.ErrClosed`. | Returns `scheduler.ErrClosed` for cron registration APIs. |
| Register handler after start | Returns `scheduler.ErrAfterStart`. | Returns `scheduler.ErrAfterStart`. |

## Example

```go
redisClient, err := redisconn.NewClient(conf.Redis)
if err != nil {
	return nil, err
}
s, err := asynq.NewScheduler(conf.Scheduler, asynq.WithClient(redisClient))
if err != nil {
	return nil, err
}
if err := s.Handle("user.email.welcome", sendWelcomeEmail); err != nil {
	return nil, err
}
if err := s.Register("daily-cleanup", "0 3 * * *", dailyCleanup); err != nil {
	return nil, err
}
return []task.Task{s, httpServer}, nil
```
