package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Extra struct {
	Key  string `json:"key"`
	Vals string `json:"vals"`
}

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Unique().Immutable().Comment("用户ID"),
		field.String("username").Comment("用户名").MinLen(1),
		field.String("remark").Optional().Default("").Comment("备注").MaxLen(30),
		field.String("avatar").Comment("头像").Default(""),
		field.String("phone").Optional().Default("").Comment("手机号").MaxLen(20),
		field.Uint64("flags").Default(0).Comment("标记位"),
		field.JSON("extra", Extra{}).Optional().Comment("额外信息"),
	}
}
