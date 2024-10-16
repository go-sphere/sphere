package render

import (
	"github.com/tbxark/sphere/internal/pkg/database/ent"
)

type Admin struct {
	ID       int      `json:"id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles,omitempty"`
}

func (r *Render) Admin(a *ent.Admin) *Admin {
	if a == nil {
		return nil
	}
	return &Admin{
		ID:       a.ID,
		Username: a.Username,
	}
}

func (r *Render) AdminWithRoles(a *ent.Admin) *Admin {
	return &Admin{
		ID:       a.ID,
		Username: a.Username,
		Roles:    a.Roles,
	}
}
