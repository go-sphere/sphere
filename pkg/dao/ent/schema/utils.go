package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	"time"
)

const DefaultTargetPage = "/pages/error/404"

type KeyValueItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func TimestampDefaultFunc() int64 {
	return time.Now().Unix()
}

func KeyValueItemDefaultFunc() []KeyValueItem {
	return []KeyValueItem{}
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
