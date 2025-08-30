package log

import (
	"time"

	"go.uber.org/zap"
)

// Field is an alias for zap.Field providing structured logging fields.
type Field = zap.Field

// String creates a structured field with a string value.
func String(key, value string) Field {
	return zap.String(key, value)
}

// Int creates a structured field with an integer value.
func Int(key string, value int) Field {
	return zap.Int(key, value)
}

// Int64 creates a structured field with a 64-bit integer value.
func Int64(key string, value int64) Field {
	return zap.Int64(key, value)
}

// Uint64 creates a structured field with an unsigned 64-bit integer value.
func Uint64(key string, value uint64) Field {
	return zap.Uint64(key, value)
}

// Float64 creates a structured field with a floating-point value.
func Float64(key string, value float64) Field {
	return zap.Float64(key, value)
}

// Bool creates a structured field with a boolean value.
func Bool(key string, value bool) Field {
	return zap.Bool(key, value)
}

// Time creates a structured field with a time value.
func Time(key string, value time.Time) Field {
	return zap.Time(key, value)
}

// Duration creates a structured field with a duration value.
func Duration(key string, value time.Duration) Field {
	return zap.Duration(key, value)
}

// Group creates a structured field that groups multiple values under a single key.
func Group(key string, values ...any) Field {
	return zap.Any(key, values)
}

// Any creates a structured field with an arbitrary value.
// The value will be serialized using the best available method.
func Any(key string, value interface{}) Field {
	return zap.Any(key, value)
}

// Err creates a structured field for error values using the standard "error" key.
func Err(err error) Field {
	return zap.Error(err)
}
