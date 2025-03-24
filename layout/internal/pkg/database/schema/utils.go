package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

func TimestampDefaultFunc() int64 {
	return time.Now().Unix()
}

func DefaultTimeFields() [2]ent.Field {
	return [2]ent.Field{
		field.Int64("created_at").
			Optional().
			Immutable().
			DefaultFunc(TimestampDefaultFunc).
			Comment("创建时间"),
		field.Int64("updated_at").
			Optional().
			DefaultFunc(TimestampDefaultFunc).
			UpdateDefault(TimestampDefaultFunc).
			Comment("更新时间"),
	}
}

type TimeMixin struct {
	mixin.Schema
}

func (TimeMixin) Fields() []ent.Field {
	fields := DefaultTimeFields()
	return []ent.Field{fields[0], fields[1]}
}
