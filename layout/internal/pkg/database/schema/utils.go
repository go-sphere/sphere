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

// DefaultTimeFields returns the default fields for created_at and updated_at.
// If TimeMixin is used directly, the generated proto file will place these two fields at the very beginning.
// This is a bit strange, so we manually create these two fields and insert them where needed.
// This way, the generated proto file can place these two fields in the desired position.
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
