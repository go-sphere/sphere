// Package captcha issues one-time verification codes (SMS, email) with
// per-number rate limits and brute-force lockout. It is experimental and
// keeps all state in process memory; nothing is persisted or shared between
// replicas.
//
// [NewManager] combines code generation, a caller-supplied [Sender], and a
// [VerificationSystem]. Manager implements task.Task: run it (for example by
// returning it from a boot builder) so Start's once-a-minute cleanup keeps
// the maps bounded, and Stop it on shutdown.
//
// # Usage
//
//	import (
//		"context"
//
//		"github.com/go-sphere/sphere/utils/exp/captcha"
//	)
//
//	manager := captcha.NewManager(captcha.Config{}, smsSender) // all defaults
//	go func() { _ = manager.Start(ctx) }()
//	defer manager.Stop(context.Background())
//
//	if err := manager.SendCode("+8613800000000"); err != nil {
//		// errors.Is(err, captcha.ErrMinuteLimitExceeded) or
//		// captcha.ErrDailyLimitExceeded when rate limited; otherwise the
//		// sender failed.
//		return err
//	}
//	ok := manager.Verify("+8613800000000", submittedCode)
//
// # Rules
//
//   - Defaults: code length 6, expiry 300s, 1 send per minute and 100 per
//     day per number, 5 failed verifications then a 15-minute freeze.
//     Non-positive config values take these defaults.
//   - Verify consumes a matching code. During a freeze outstanding codes are
//     kept, not invalidated; issuing a new code lifts the freeze.
//   - Empty codes are never stored or accepted.
//   - If storing succeeds but the Sender fails, the code stays stored and the
//     send still counts against the quota.
//   - RandomCode uses crypto/rand and panics on entropy failure.
//
// Manager and VerificationSystem are safe for concurrent use.
package captcha
