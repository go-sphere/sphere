package tasktest_test

import (
	"testing"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/scripttask"
	"github.com/go-sphere/sphere/core/task/tasktest"
)

// TestAssertLifecycleContractScriptTask runs the lifecycle contract against the
// built-in scripttask, which is expected to satisfy every guarantee.
func TestAssertLifecycleContractScriptTask(t *testing.T) {
	tasktest.AssertLifecycleContract(t, func() task.Task {
		return scripttask.NewScriptTask("contract", nil, nil)
	})
}
