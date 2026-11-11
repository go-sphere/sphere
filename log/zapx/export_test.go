package zapx

import (
	corelog "github.com/go-sphere/sphere/log"
	"go.uber.org/zap/zapcore"
)

// NewBackendFromCore exposes newBackendFromCore to the external test package,
// whose frames lie outside the log package tree as caller reporting requires.
func NewBackendFromCore(core zapcore.Core, options ...corelog.Option) *Backend {
	return newBackendFromCore(core, options...)
}
