package log

import (
	"testing"
)

func TestGroup(t *testing.T) {
	field := Group("test", 123, "abc", 3.14)
	Warn("test", field)
}
