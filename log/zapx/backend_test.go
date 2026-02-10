package zapx

import (
	"context"
	"log/slog"
	"testing"

	"github.com/go-sphere/sphere/log"
	"go.uber.org/zap"
)

func TestZapLogger(t *testing.T) {
	backend := NewBackend(NewDefaultConfig(), log.AddCaller())

	log.InitWithBackends(backend)
	log.Warn("info")
	log.Warnf("warn %s", "value")
	log.WarnContext(context.Background(), "warn context")
	log.With(log.WithAttrs(map[string]any{
		"extra": "extra value",
	})).WarnContext(context.Background(), "warn", log.String("key", "value"))

	slog.SetDefault(backend.SlogLogger(log.AddCaller()))
	slog.Warn("slog", log.String("key", "value"))
	slog.WarnContext(context.Background(), "slog", log.String("key", "value"))

	backend.ZapLogger().Warn("zap", zap.String("key", "value"))
	backend.zapLogger.With(zap.String("key", "value")).Warn("zap", zap.String("key2", "value2"))
}
