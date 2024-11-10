package schema

import (
	"database/sql/driver"
	"entgo.io/contrib/entproto"
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/descriptorpb"
	"time"
)

type Extra struct {
	Key  string `json:"key"`
	Vals string `json:"vals"`
}

type Level int

type Role string

const (
	Unknown Level = iota
	Low
	High
)

func (p Level) String() string {
	switch p {
	case Low:
		return "LOW"
	case High:
		return "HIGH"
	default:
		return "UNKNOWN"
	}
}

// Values provides list valid values for Enum.
func (Level) Values() []string {
	return []string{Unknown.String(), Low.String(), High.String()}
}

// Value provides the DB a string from int.
func (p Level) Value() (driver.Value, error) {
	return p.String(), nil
}

// Scan tells our code how to read the enum into our type.
func (p *Level) Scan(val any) error {
	var s string
	switch v := val.(type) {
	case nil:
		return nil
	case string:
		s = v
	case []uint8:
		s = string(v)
	}
	switch s {
	case "LOW":
		*p = Low
	case "HIGH":
		*p = High
	default:
		*p = Unknown
	}
	return nil
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
		field.JSON("roles", []string{}).Optional().Comment("角色列表"),
		field.JSON("extra", Extra{}).Optional().Comment("额外信息").Annotations(
			entproto.Field(6, entproto.Type(descriptorpb.FieldDescriptorProto_TYPE_BYTES)),
		),
		field.UUID("uuid", uuid.UUID{}).Default(uuid.New).Comment("UUID"),
		field.Enum("level").GoType(Level(0)),
		field.Time("created_at").Immutable().Default(time.Now()).Comment("创建时间"),
	}
}
