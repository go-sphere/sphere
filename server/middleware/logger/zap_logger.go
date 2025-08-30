package logger

import (
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Logger defines a minimal logging interface for HTTP request logging.
// It supports both info and error level logging with variadic arguments.
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

type ginZapLogger struct {
	logger Logger
}

// NewZapLoggerAdapter creates a ginzap.ZapLogger adapter from a generic Logger interface.
// This allows using any logger implementation with gin-contrib/zap middleware.
func NewZapLoggerAdapter(logger Logger) ginzap.ZapLogger {
	return &ginZapLogger{logger}
}

func (g *ginZapLogger) Info(msg string, fields ...zap.Field) {
	args := make([]interface{}, 0, len(fields))
	for _, f := range fields {
		args = append(args, f)
	}
	g.logger.Info(msg, args...)
}

func (g *ginZapLogger) Error(msg string, fields ...zap.Field) {
	args := make([]interface{}, 0, len(fields))
	for _, f := range fields {
		args = append(args, f)
	}
	g.logger.Error(msg, args...)
}

// NewLoggerMiddleware creates a Gin logging middleware using the provided logger.
// It logs HTTP requests with RFC3339 timestamp format and UTC timezone.
func NewLoggerMiddleware(logger Logger) gin.HandlerFunc {
	return ginzap.Ginzap(NewZapLoggerAdapter(logger), time.RFC3339, true)
}

// NewRecoveryMiddleware creates a Gin recovery middleware that logs panics using the provided logger.
// It recovers from panics and logs them while keeping the HTTP stack intact.
func NewRecoveryMiddleware(logger Logger) gin.HandlerFunc {
	return ginzap.RecoveryWithZap(NewZapLoggerAdapter(logger), true)
}

// NewZapLoggerMiddleware creates a Gin logging middleware using a native ginzap.ZapLogger.
// This is useful when you already have a ginzap.ZapLogger implementation.
func NewZapLoggerMiddleware(logger ginzap.ZapLogger) gin.HandlerFunc {
	return ginzap.Ginzap(logger, time.RFC3339, true)
}

func NewZapRecoveryMiddleware(logger ginzap.ZapLogger) gin.HandlerFunc {
	return ginzap.RecoveryWithZap(logger, true)
}
