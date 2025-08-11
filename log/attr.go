package log

import (
	"time"

	"go.uber.org/zap"
)

type Attr = zap.Field

func String(key, value string) Attr {
	return zap.String(key, value)
}

func Int(key string, value int) Attr {
	return zap.Int(key, value)
}

func Int64(key string, value int64) Attr {
	return zap.Int64(key, value)
}

func Uint64(key string, value uint64) Attr {
	return zap.Uint64(key, value)
}

func Float64(key string, value float64) Attr {
	return zap.Float64(key, value)
}

func Bool(key string, value bool) Attr {
	return zap.Bool(key, value)
}

func Time(key string, value time.Time) Attr {
	return zap.Time(key, value)
}

func Duration(key string, value time.Duration) Attr {
	return zap.Duration(key, value)
}

func Group(key string, attrs ...any) Attr {
	return zap.Any(key, attrs)
}

func Any(key string, value interface{}) Attr {
	return zap.Any(key, value)
}

func Err(err error) Attr {
	return zap.Error(err)
}
