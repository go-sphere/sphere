package tags

import (
	"testing"
)

func TestNewSphereTagItems(t *testing.T) {
	items := NewSphereTagItems("form,!json", "name")
	t.Logf("%s", items.Format())

	items = NewSphereTagItems(`form,uri="demo"`, "name")
	t.Logf("%s", items.Format())
}
