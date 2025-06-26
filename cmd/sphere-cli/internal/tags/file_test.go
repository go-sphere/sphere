package tags

import "testing"

func TestReTags(t *testing.T) {
	err := ReTags("../../../../layout/api/*/*/test.pb.go", true, true)
	if err != nil {
		t.Errorf("ReTags failed: %v", err)
	} else {
		t.Logf("ReTags completed successfully")
	}
}
