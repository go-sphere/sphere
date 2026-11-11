//go:build unix

package boot

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task"
)

// signalHelperEnv switches TestRunSecondSignalTerminatesHungShutdown into the
// child process it re-executes.
const signalHelperEnv = "SPHERE_BOOT_SIGNAL_HELPER"

// TestRunSecondSignalTerminatesHungShutdown pins that once a shutdown signal
// has been received, a second one is no longer swallowed by Run: it gets the
// default action and kills a process whose Stop never returns. Signals are
// process-wide, so the scenario runs in a re-executed child.
func TestRunSecondSignalTerminatesHungShutdown(t *testing.T) {
	if os.Getenv(signalHelperEnv) == "1" {
		runHungShutdownHelper()
		return
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() { _ = pr.Close() }()

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunSecondSignalTerminatesHungShutdown$")
	cmd.Env = append(os.Environ(), signalHelperEnv+"=1")
	cmd.Stdout = pw
	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		t.Fatalf("start helper: %v", err)
	}
	_ = pw.Close()

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	helperExited := false
	t.Cleanup(func() {
		if helperExited {
			return
		}
		_ = cmd.Process.Kill()
		if err := <-exited; err != nil {
			t.Logf("killed helper: %v", err)
		}
	})

	lines := make(chan string, 16)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	waitForLine := func(want string) {
		t.Helper()
		deadline := time.After(10 * time.Second)
		for {
			select {
			case line, ok := <-lines:
				if !ok {
					t.Fatalf("helper output ended before %q", want)
				}
				if line == want {
					return
				}
			case <-deadline:
				t.Fatalf("helper did not print %q", want)
			}
		}
	}

	waitForLine("HELPER_STARTED")
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("first SIGINT: %v", err)
	}
	waitForLine("HELPER_STOPPING")
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("second SIGINT: %v", err)
	}

	select {
	case err := <-exited:
		helperExited = true
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("helper exited with %v, want termination by SIGINT", err)
		}
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if !ok || !status.Signaled() || status.Signal() != syscall.SIGINT {
			t.Fatalf("helper exit status = %v, want termination by SIGINT", exitErr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("second SIGINT did not terminate a hung shutdown")
	}
}

// runHungShutdownHelper runs an application whose Stop never returns under an
// unbounded shutdown timeout, so only a signal can end the process.
func runHungShutdownHelper() {
	hung := task.NewFunc("hung", func(ctx context.Context) error {
		fmt.Println("HELPER_STARTED")
		<-ctx.Done()
		return nil
	}, func(context.Context) error {
		fmt.Println("HELPER_STOPPING")
		time.Sleep(time.Hour)
		return nil
	})
	_ = Run(new(struct{}), func(*struct{}) (*Application, error) {
		return NewApplication(hung), nil
	}, WithShutdownTimeout(0), WithShutdownSignals(syscall.SIGINT))
	os.Exit(0)
}
