package cron_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/scheduler"
	"github.com/go-sphere/sphere/scheduler/cron"
)

// Register every job before Start. A scheduler that is never started is
// released with Close; a started one is stopped by its task runner.
func ExampleScheduler_Register() {
	s, err := cron.NewScheduler(cron.Config{Seconds: true, Timezone: "UTC"})
	if err != nil {
		panic(err)
	}
	job := func(context.Context) error { return nil }

	if err := s.Register("report", "0 */5 * * * *", job); err != nil {
		panic(err)
	}
	err = s.Register("report", "0 0 * * * *", job)
	fmt.Println(errors.Is(err, scheduler.ErrDuplicateName))

	if err := s.Close(); err != nil {
		panic(err)
	}
	err = s.Register("late", "0 0 * * * *", job)
	fmt.Println(errors.Is(err, scheduler.ErrClosed))
	// Output:
	// true
	// true
}
