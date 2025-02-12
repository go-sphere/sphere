package logfields

import "go.uber.org/zap"

type Field = zap.Field

func String(k, v string) Field {
	return zap.String(k, v)
}

func Int(k string, v int) Field {
	return zap.Int(k, v)
}

func Error(err error) Field {
	return zap.Error(err)
}

func Any(k string, v interface{}) Field {
	return zap.Any(k, v)
}
