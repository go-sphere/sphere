package bind

import (
	"testing"
)

type Admin struct {
	ID   int
	Name string
}

type AdminPb struct {
	ID   *int64
	Name *string
}

type AdminCreate struct {
	id   int
	name string
}

func (a *AdminCreate) SetID(id int) {
	a.id = id
}

func (a *AdminCreate) SetNillableID(id *int) {
	if id == nil {
		a.id = 0
	} else {
		a.id = *id
	}
}

func (a *AdminCreate) SetName(name string) {
	a.name = name
}

func (a *AdminCreate) SetNillableName(name *string) {
	if name == nil {
		a.name = ""
	} else {
		a.name = *name
	}
}

func (a *AdminCreate) ClearName() {
	a.name = ""
}

func TestGenBindFunc(t *testing.T) {
	file, err := GenFile(&GenFileConf{
		Entities: []GenFileEntityConf{
			{
				Actions: []any{AdminCreate{}},
				ConfigBuilder: func(act any) *GenFuncConf {
					return NewGenFuncConf(Admin{}, AdminPb{}, act)
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to generate bind function: %v", err)
	}
	t.Logf("Generated bind function:\n%s", file)
}
