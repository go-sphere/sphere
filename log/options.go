package log

type FileOptions struct {
	FileName   string `json:"file_name" yaml:"file_name"`
	MaxSize    int    `json:"max_size" yaml:"max_size"`
	MaxBackups int    `json:"max_backups" yaml:"max_backups"`
	MaxAge     int    `json:"max_age" yaml:"max_age"`
}

type ConsoleOptions struct {
	AsyncOut bool `json:"async_out" yaml:"async_out"`
}

type Options struct {
	File    *FileOptions    `json:"file" yaml:"file"`
	Console *ConsoleOptions `json:"console" yaml:"console"`
	Level   string          `json:"level" yaml:"level"`
}

func NewOptions() *Options {
	return &Options{
		File: nil,
		Console: &ConsoleOptions{
			AsyncOut: true,
		},
		Level: "info",
	}
}
