package log

import (
	"fmt"
	"os"
)

func Warn(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, "\u001B[31mWARN\u001B[m: "+format+"\n", args...)
}
