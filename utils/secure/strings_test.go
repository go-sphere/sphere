package secure

import "testing"

func TestCensorString(t *testing.T) {
	t.Log(CensorString("test", 4))
	t.Log(CensorString("test", 5))
	t.Log(CensorString("test", 6))
}
