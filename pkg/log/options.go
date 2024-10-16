package log

type FileOptions struct {
	FileName   string `json:"file_name" yaml:"file_name"`
	MaxSize    int    `json:"max_size" yaml:"max_size"`
	MaxBackups int    `json:"max_backups" yaml:"max_backups"`
	MaxAge     int    `json:"max_age" yaml:"max_age"`
}

type Options struct {
	File            *FileOptions `json:"file" yaml:"file"`
	ConsoleOutAsync bool         `json:"console_out_async" yaml:"console_out_async"`
	Level           string       `json:"level" yaml:"level"`
}

func NewOptions() *Options {
	return &Options{
		File:            nil,
		ConsoleOutAsync: true,
		Level:           "info",
	}
}
