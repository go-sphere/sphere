package zapx

import (
	corelog "github.com/go-sphere/sphere/log"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

const defaultLevel = "info"

// Config defines zap backend output and level settings. Level is a zap
// level string (default "info"); an invalid value falls back to Info.
// log.WithMinLevel is ignored; set Level instead.
type Config struct {
	File    FileConfig    `json:"file" yaml:"file"`
	Console ConsoleConfig `json:"console" yaml:"console"`
	Level   string        `json:"level" yaml:"level"`
}

// ConsoleConfig configures console logging output.
// When Disable is true, console logging is completely turned off.
type ConsoleConfig struct {
	Disable bool `json:"disable" yaml:"disable"`
}

// FileConfig configures lumberjack JSON file rotation. An empty FileName
// disables the file sink.
type FileConfig struct {
	FileName string `json:"file_name" yaml:"file_name"`
	// MaxSize is the rotation threshold in megabytes.
	MaxSize int `json:"max_size" yaml:"max_size"`
	// MaxBackups is the number of old log files to keep.
	MaxBackups int `json:"max_backups" yaml:"max_backups"`
	// MaxAge is the maximum age of an old log file in days.
	MaxAge int `json:"max_age" yaml:"max_age"`
}

// NewDefaultConfig creates a config with info level logging and no file output.
func NewDefaultConfig() Config {
	return Config{
		File:    FileConfig{},
		Console: ConsoleConfig{},
		Level:   defaultLevel,
	}
}

func zapOptions(o corelog.Options) []zap.Option {
	opts := make([]zap.Option, 0, 3)
	switch o.AddCaller {
	case corelog.AddCallerStatusEnable:
		opts = append(opts, zap.WithCaller(true))
	case corelog.AddCallerStatusDisable:
		opts = append(opts, zap.WithCaller(false))
	default:
		break
	}
	if o.AddStackAt != nil {
		opts = append(opts, zap.AddStacktrace(logLevelToZapLevel(*o.AddStackAt)))
	}
	return opts
}
func zapSlogOptions(o corelog.Options) []zapslog.HandlerOption {
	opts := make([]zapslog.HandlerOption, 0, 4)
	switch o.AddCaller {
	case corelog.AddCallerStatusEnable:
		opts = append(opts, zapslog.WithCaller(true))
	case corelog.AddCallerStatusDisable:
		opts = append(opts, zapslog.WithCaller(false))
	default:
		break
	}
	if o.Name != "" {
		opts = append(opts, zapslog.WithName(o.Name))
	}
	if o.AddStackAt != nil {
		opts = append(opts, zapslog.AddStacktraceAt(logLevelToSlogLevel(*o.AddStackAt)))
	}
	return opts
}
