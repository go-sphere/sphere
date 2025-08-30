package safe

import (
	"testing"
)

func TestDeferIfErrorPresent(t *testing.T) {
	t.Logf("1")
	defer IfErrorPresent(func() error {
		t.Logf("5")
		return nil
	})
	t.Logf("2")
	defer IfErrorPresent(func() error {
		t.Logf("4")
		return nil
	})
	t.Logf("3")
}

func TestDefer2IfErrorPresent(t *testing.T) {
	t.Logf("1")
	defer func() {
		t.Logf("5")
	}()
	t.Logf("2")
	defer func() {
		t.Logf("4")
	}()
	t.Logf("3")
}
