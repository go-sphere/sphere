package render

import (
	"github.com/TBXark/sphere/database/mapper"
	"github.com/TBXark/sphere/layout/api/entpb"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
)

func (r *Render) AdminSession(value *ent.AdminSession) *entpb.AdminSession {
	res := mapper.MapStruct[ent.AdminSession, entpb.AdminSession](value)
	return res
}

func (r *Render) KeyValueStore(value *ent.KeyValueStore) *entpb.KeyValueStore {
	res := mapper.MapStruct[ent.KeyValueStore, entpb.KeyValueStore](value)
	return res
}
