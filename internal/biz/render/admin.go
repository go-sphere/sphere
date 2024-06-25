package render

import (
	"github.com/tbxark/go-base-api/pkg/dao/ent"
)

type Admin struct {
	ID         int      `json:"id"`
	Username   string   `json:"username"`
	Permission []string `json:"permission,omitempty"`
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

func AdminWithPermission(a *ent.Admin) *Admin {
	return &Admin{
		ID:         a.ID,
		Username:   a.Username,
		Permission: a.Permission,
	}
}
