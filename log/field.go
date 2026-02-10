package log

import (
	"log/slog"
	"time"
)

// Attr is a strongly-typed structured log field based on slog's value model.
type Attr = slog.Attr

// Field is kept as an alias for backward compatibility.
type Field = Attr

// String creates a structured field with a string value.
func String(key, value string) Attr {
	return slog.String(key, value)
}

// Int creates a structured field with an integer value.
func Int(key string, value int) Attr {
	return slog.Int(key, value)
}

// Int64 creates a structured field with a 64-bit integer value.
func Int64(key string, value int64) Attr {
	return slog.Int64(key, value)
}

// Uint64 creates a structured field with an unsigned 64-bit integer value.
func Uint64(key string, value uint64) Attr {
	return slog.Uint64(key, value)
}

// Float64 creates a structured field with a floating-point value.
func Float64(key string, value float64) Attr {
	return slog.Float64(key, value)
}

// Bool creates a structured field with a boolean value.
func Bool(key string, value bool) Attr {
	return slog.Bool(key, value)
}

// Time creates a structured field with a time value.
func Time(key string, value time.Time) Attr {
	return slog.Time(key, value)
}

// Duration creates a structured field with a duration value.
func Duration(key string, value time.Duration) Attr {
	return slog.Duration(key, value)
}

// Group creates a structured field that groups multiple attrs under a single key.
func Group(key string, attrs ...Attr) Attr {
	return Attr{Key: key, Value: slog.GroupValue(attrs...)}
}

// Any creates a structured field with an arbitrary value.
func Any(key string, value any) Attr {
	return slog.Any(key, value)
}

// Err creates a structured field for error values using the standard "error" key.
func Err(err error) Attr {
	return Any("error", err)
}
