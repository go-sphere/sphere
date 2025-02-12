package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Task struct {
	ent.Schema
}

func (Task) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("job_id", uuid.UUID{}).Unique().Immutable().Comment("任务ID"),
		field.String("name").Default("").Comment("任务名称"),
		field.Enum("status").Values("pending", "running", "success", "failed").Default("pending").Comment("任务状态"),
		field.String("result").Optional().Default("").MaxLen(1024).Comment("任务结果"),
		field.String("error").Optional().Default("").MaxLen(1024).Comment("错误信息"),
	}
}

func (Task) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}
