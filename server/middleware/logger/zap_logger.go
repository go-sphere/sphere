package logger

import (
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

type ginZapLogger struct {
	logger Logger
}

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

func NewLoggerMiddleware(logger Logger) gin.HandlerFunc {
	return ginzap.Ginzap(NewZapLoggerAdapter(logger), time.RFC3339, true)
}

func NewRecoveryMiddleware(logger Logger) gin.HandlerFunc {
	return ginzap.RecoveryWithZap(NewZapLoggerAdapter(logger), true)
}

func NewZapLoggerMiddleware(logger ginzap.ZapLogger) gin.HandlerFunc {
	return ginzap.Ginzap(logger, time.RFC3339, true)
}

func NewZapRecoveryMiddleware(logger ginzap.ZapLogger) gin.HandlerFunc {
	return ginzap.RecoveryWithZap(logger, true)
}
