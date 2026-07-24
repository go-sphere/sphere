package captcha

import (
	"testing"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
)

type noopSender struct{}

func (noopSender) SendCode(string, string) error {
	return nil
}

func TestManagerLifecycleContract(t *testing.T) {
	tasktest.AssertLifecycleContract(t, func() task.Task {
		return NewManager(Config{}, noopSender{})
	})
}
