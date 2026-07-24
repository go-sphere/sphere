package qiniu

import (
	"testing"

	qiniuStorage "github.com/qiniu/go-sdk/v7/storage"
)

func TestIsNotFoundError(t *testing.T) {
	t.Parallel()

	err := &qiniuStorage.ErrorInfo{Code: 612}
	if !isNotFoundError(err) {
		t.Fatalf("expected Qiniu error code 612 to be recognized as not found")
	}
}
