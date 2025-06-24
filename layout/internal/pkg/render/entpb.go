package render

import (
	"github.com/TBXark/sphere/database/mapper"
	"github.com/TBXark/sphere/layout/api/entpb"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
)

func (r *Render) AdminSession(voice *ent.AdminSession) *entpb.AdminSession {
	res := mapper.MapStruct[ent.AdminSession, entpb.AdminSession](voice)
	return res
}
