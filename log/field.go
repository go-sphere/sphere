package log

import (
	"time"

	"go.uber.org/zap"
)

type Field = zap.Field

func String(key, value string) Field {
	return zap.String(key, value)
}

func Int(key string, value int) Field {
	return zap.Int(key, value)
}

func Int64(key string, value int64) Field {
	return zap.Int64(key, value)
}

func Uint64(key string, value uint64) Field {
	return zap.Uint64(key, value)
}

func Float64(key string, value float64) Field {
	return zap.Float64(key, value)
}

func Bool(key string, value bool) Field {
	return zap.Bool(key, value)
}

func Time(key string, value time.Time) Field {
	return zap.Time(key, value)
}

func Duration(key string, value time.Duration) Field {
	return zap.Duration(key, value)
}

func Group(key string, values ...any) Field {
	return zap.Any(key, values)
}

func Any(key string, value interface{}) Field {
	return zap.Any(key, value)
}

func Err(err error) Field {
	return zap.Error(err)
}
