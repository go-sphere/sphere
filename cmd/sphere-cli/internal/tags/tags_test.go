package tags

import (
	"testing"
)

func TestNewSphereTagItems(t *testing.T) {
	items := NewSphereTagItems(FromComment(`//@sphere:"form,!json"`), "name")
	t.Logf("%s", items.Format())

	items = NewSphereTagItems(FromComment(`//@sphere:form,uri="demo"`), "name")
	t.Logf("%s", items.Format())
}
