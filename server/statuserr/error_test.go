package statuserr

import (
	"errors"
	"os"
	"testing"
)

func TestJoinError(t *testing.T) {
	err := JoinError(404, "找不到文件", os.ErrNotExist)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected error to not be os.ErrNotExist, got %v", err)
	}
}
