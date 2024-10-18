package render

import (
	dashv1 "github.com/tbxark/sphere/api/dash/v1"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
)

func (r *Render) Admin(a *ent.Admin) *dashv1.Admin {
	if a == nil {
		return nil
	}
	return &dashv1.Admin{
		Id:       a.ID,
		Username: a.Username,
	}
}

func (r *Render) AdminWithRoles(a *ent.Admin) *dashv1.Admin {
	return &dashv1.Admin{
		Id:       a.ID,
		Username: a.Username,
		Roles:    a.Roles,
	}
}
