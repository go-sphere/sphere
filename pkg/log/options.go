package log

type FileOptions struct {
	FileName   string `json:"file_name"`
	MaxSize    int    `json:"max_size"`
	MaxBackups int    `json:"max_backups"`
	MaxAge     int    `json:"max_age"`
}

type Options struct {
	File            *FileOptions `json:"file"`
	ConsoleOutAsync bool         `json:"console_out_async"`
	Level           string       `json:"level"`
}

func NewOptions() *Options {
	return &Options{
		File:            nil,
		ConsoleOutAsync: true,
		Level:           "info",
	}
}
