package asynq

// periodic.go is intentionally small for now. Periodic tasks are registered
// directly through asynq.Scheduler in this first implementation; a dynamic
// PeriodicTaskConfigProvider adapter can be added here without changing the
// public scheduler interface.
