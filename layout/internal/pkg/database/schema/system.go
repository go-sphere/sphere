package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type KeyValueStore struct {
	ent.Schema
}

func (KeyValueStore) Fields() []ent.Field {
	times := DefaultTimeFields()
	return []ent.Field{
		field.String("key").Unique().Comment("键"),
		field.Bytes("value").Optional().Comment("值"),
		times[0], times[1],
	}
}
