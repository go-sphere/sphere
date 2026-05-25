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

## Error Semantics

| Scenario | asynq driver | cron driver |
| --- | --- | --- |
| Handler returns error | Uses asynq retry and archive behavior according to enqueue options. | Logs the error once and does not retry. |
| Handler panics | Panic is recovered, logged, and returned as an error so asynq can apply retry policy. | Panic is recovered and logged by cron middleware; no retry is attempted. |
| Task kind has no handler | asynq treats it as an unhandled task and applies its normal failure behavior. | Not applicable. |
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
