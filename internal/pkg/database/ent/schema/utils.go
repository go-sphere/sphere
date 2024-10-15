package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"github.com/tbxark/go-base-api/internal/pkg/database/ent/schema/types"
	"time"
)

func TimestampDefaultFunc() int64 {
	return time.Now().Unix()
}

func KeyValueItemDefaultFunc() []types.KeyValueItem {
	return []types.KeyValueItem{}
}

type TimeMixin struct {
	mixin.Schema
}

func (TimeMixin) Fields() []ent.Field {
	return []ent.Field{
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
