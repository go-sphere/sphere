// Package logbuffer is an in-memory, cursor-addressable log tail for live
// log views such as an admin SSE endpoint.
//
// [Buffer] is a [github.com/go-sphere/sphere/log.Backend]. Create it with
// [New], install it with log.InitWithBackends next to the real output
// backends, and it captures every entry into a fixed-size ring with a
// monotonically increasing sequence number. [Buffer.Subscribe] streams live
// entries; [Buffer.History] pages backwards through the retained tail. A
// Buffer needs no cleanup beyond cancelling each [Subscription].
//
// # Usage
//
//	import (
//		"github.com/go-sphere/sphere/log"
//		"github.com/go-sphere/sphere/log/logbuffer"
//	)
//
//	buf := logbuffer.New(1000)
//	log.InitWithBackends(log.NewStdioBackend(), buf)
//
//	sub := buf.Subscribe(logbuffer.SubscribeOptions{
//		StreamID:  clientStreamID, // last buf.ID() the client saw, or ""
//		FromSeq:   clientLastSeq,  // last Seq the client saw, or 0
//		TailLimit: 100,            // backfill size when FromSeq is 0
//	})
//	defer sub.Cancel()
//	for _, e := range sub.Backfill {
//		send(e)
//	}
//	for {
//		select {
//		case <-ctx.Done():
//			return
//		case e := <-sub.C:
//			send(e)
//		}
//	}
//
// # Continuity
//
// The sequence number is the correctness anchor. Subscribe atomically
// returns the backfill after a cursor together with the live channel, so
// there is no loss window between history and live entries. A cursor that
// has fallen out of the ring, and entries dropped on a slow subscriber, are
// reported explicitly (Subscription.Truncated, [Subscription.Dropped])
// instead of being silently lost.
//
// Sequence numbers restart at 1 with each new Buffer. Pair resume cursors
// with [Buffer.ID]; Subscribe reports Subscription.Reset when a cursor
// belongs to an earlier Buffer instance.
package logbuffer
