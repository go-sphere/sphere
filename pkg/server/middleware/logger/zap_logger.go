package logger

import (
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"time"
)

type Logger interface {
	Infow(msg string, args ...interface{})
	Errorw(msg string, args ...interface{})
}

type ginZapLogger struct {
	logger Logger
}

func NewGinZapLogger(logger Logger) ginzap.ZapLogger {
	return &ginZapLogger{logger}
}

func (g *ginZapLogger) Info(msg string, fields ...zap.Field) {
	args := make([]interface{}, 0, len(fields))
	for _, f := range fields {
		args = append(args, f)
	}
	g.logger.Infow(msg, args...)
}

func (g *ginZapLogger) Error(msg string, fields ...zap.Field) {
	args := make([]interface{}, 0, len(fields))
	for _, f := range fields {
		args = append(args, f)
	}
	g.logger.Errorw(msg, args...)
}

func NewLoggerMiddleware(logger Logger) gin.HandlerFunc {
	return ginzap.Ginzap(NewGinZapLogger(logger), time.RFC3339, true)
}

func NewRecoveryMiddleware(logger Logger) gin.HandlerFunc {
	return ginzap.RecoveryWithZap(NewGinZapLogger(logger), true)
}

func NewZapLoggerMiddleware(logger ginzap.ZapLogger) gin.HandlerFunc {
	return ginzap.Ginzap(logger, time.RFC3339, true)
}

func NewZapRecoveryMiddleware(logger ginzap.ZapLogger) gin.HandlerFunc {
	return ginzap.RecoveryWithZap(logger, true)
}
