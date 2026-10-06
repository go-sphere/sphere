package logbuffer_test

import (
	"fmt"

	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/log/logbuffer"
)

// Resume a tail from a cursor: Backfill holds what the client missed and C
// delivers every later entry without a gap.
func ExampleBuffer_Subscribe() {
	buf := logbuffer.New(100)
	logger := log.NewLogger(buf) // in a service: log.InitWithBackends(output, buf)

	logger.Info("first")
	logger.Info("second")

	sub := buf.Subscribe(logbuffer.SubscribeOptions{
		StreamID: buf.ID(),
		FromSeq:  1, // the client already saw entry 1
	})
	defer sub.Cancel()

	for _, e := range sub.Backfill {
		fmt.Println("backfill", e.Seq, e.Message)
	}
	logger.Warn("third", log.String("disk", "sda"))
	e := <-sub.C
	fmt.Println("live", e.Seq, e.Level, e.Message, e.Attrs["disk"])
	fmt.Println("truncated", sub.Truncated, "dropped", sub.Dropped())

	// Output:
	// backfill 2 second
	// live 3 warn third sda
	// truncated false dropped 0
}

// Page backwards through the retained tail, newest page first.
func ExampleBuffer_History() {
	buf := logbuffer.New(10)
	logger := log.NewLogger(buf)
	for i := 1; i <= 5; i++ {
		logger.Infof("entry %d", i)
	}

	page, more := buf.History(0, 2, log.LevelDebug)
	for _, e := range page {
		fmt.Println(e.Seq, e.Message)
	}
	fmt.Println("more:", more)

	page, more = buf.History(page[0].Seq, 2, log.LevelDebug)
	for _, e := range page {
		fmt.Println(e.Seq, e.Message)
	}
	fmt.Println("more:", more)

	// Output:
	// 4 entry 4
	// 5 entry 5
	// more: true
	// 2 entry 2
	// 3 entry 3
	// more: true
}
