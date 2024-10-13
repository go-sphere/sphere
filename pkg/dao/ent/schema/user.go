package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tbxark/go-base-api/pkg/dao/idgenerator"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").Unique().Immutable().DefaultFunc(idgenerator.NextId).Comment("用户ID"),
		field.String("username").Comment("用户名").MinLen(1),
		field.String("remark").Optional().Default("").Comment("备注").MaxLen(30),
		field.String("avatar").Comment("头像"),
		field.String("phone").
			Optional().
			Default("").
			Comment("手机号").
			MaxLen(20),
		field.Uint64("flags").Default(0).Comment("标记位"),
	}
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

type UserPlatform struct {
	ent.Schema
}

func (UserPlatform) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Comment("用户ID"),
		field.String("platform").Comment("平台"),
		field.String("platform_id").Comment("平台ID"),
		field.String("second_id").Optional().Default("").Comment("第二ID"),
		//field.String("private_key").Optional().Default("").Comment("私钥").Sensitive(),
	}
}

func (UserPlatform) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("platform", "platform_id"),
	}
}

func (UserPlatform) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}
