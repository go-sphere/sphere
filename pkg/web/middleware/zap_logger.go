package middleware

import (
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/pkg/log"
	"go.uber.org/zap"
	"time"
)

type ginZapLogger struct {
	logger log.Logger
}

func NewGinZapLogger(logger log.Logger) ginzap.ZapLogger {
	return &ginZapLogger{logger}
}

func (g *ginZapLogger) Info(msg string, fields ...zap.Field) {
	g.logger.Infow(msg, fields)
}

func (g *ginZapLogger) Error(msg string, fields ...zap.Field) {
	g.logger.Errorw(msg, fields)
}

func NewZapLoggerMiddleware(logger log.Logger) gin.HandlerFunc {
	return ginzap.Ginzap(NewGinZapLogger(logger), time.RFC3339, true)
}

func NewZapRecoveryMiddleware(logger log.Logger) gin.HandlerFunc {
	return ginzap.RecoveryWithZap(NewGinZapLogger(logger), true)
}
