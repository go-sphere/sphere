// Package logger provides httpx access-log and panic-recovery middleware that
// write to a log.BaseLogger.
//
// [Log] records one entry per request after the downstream chain: Info when
// it succeeds, Error when it returns an error. [RecoveryLog] recovers a panic
// so the process stays up, logs it at Error, and finishes the request as HTTP
// 500 with no body; http.ErrAbortHandler is re-panicked, as in
// httpz.WithRecover.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/httpx/stdx"
//		"github.com/go-sphere/sphere/log"
//		"github.com/go-sphere/sphere/server/middleware/logger"
//	)
//
//	lg := log.With(log.WithName("http"))
//	engine := stdx.New()
//	engine.Use(logger.Log(lg), logger.RecoveryLog(lg, true))
//
// Register RecoveryLog inside Log, as above. RecoveryLog deliberately returns
// nil after recovering (returning an error would let an enclosing middleware
// write a second response onto the committed one), so the outer Log sees a
// successful chain and records the request at Info with status=500; the panic
// itself, with its stack, is logged at Error by RecoveryLog. Do not expect
// Log's level alone to reveal a recovered panic.
//
// Handlers wrapped by server/httpz (WithJson and friends) already recover
// their own panics and render errors, so for them Log records the written
// status at Info.
package logger
