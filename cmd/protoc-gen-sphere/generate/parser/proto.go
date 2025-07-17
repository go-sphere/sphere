package parser

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func FindProtoField(message *protogen.Message, keypath []string) *protogen.Field {
	if len(keypath) == 0 || message == nil {
		return nil
	}

	for _, field := range message.Fields {
		if string(field.Desc.Name()) != keypath[0] {
			continue
		}

		if len(keypath) == 1 {
			return field
		}

		if field.Message != nil {
			return FindProtoField(field.Message, keypath[1:])
		}

		if field.Oneof != nil {
			for _, oneofField := range field.Oneof.Fields {
				if string(oneofField.Desc.Name()) == keypath[1] {
					if len(keypath) == 2 {
						return oneofField
					}
					if len(keypath) > 2 && oneofField.Message != nil {
						return FindProtoField(oneofField.Message, keypath[2:])
					}
				}
			}
		}
	}
	return nil
}

func ProtoKeyPath2GoKeyPath(message *protogen.Message, keypath []string) []string {
	if len(keypath) == 0 || message == nil {
		return nil
	}
	goKeyPath := make([]string, 0, len(keypath))
	for _, key := range keypath {
		field := FindProtoField(message, []string{key})
		if field == nil {
			return nil
		}
		goKeyPath = append(goKeyPath, field.GoName)
		message = field.Message
	}
	return goKeyPath
}

func ProtoTypeToGoType(g *protogen.GeneratedFile, field *protogen.Field) string {
	switch {
	case field.Desc.IsMap():
		key := singularProtoTypeToGoType(g, field.Message.Fields[0])
		val := singularProtoTypeToGoType(g, field.Message.Fields[1])
		return fmt.Sprintf("map[%s]%s", key, val)
	case field.Desc.IsList():
		elemType := singularProtoTypeToGoType(g, field)
		return fmt.Sprintf("[]%s", elemType)
	default:
		return singularProtoTypeToGoType(g, field)
	}
}

func singularProtoTypeToGoType(g *protogen.GeneratedFile, field *protogen.Field) string {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind:
		return "int32"
	case protoreflect.Sint32Kind:
		return "int32"
	case protoreflect.Uint32Kind:
		return "uint32"
	case protoreflect.Int64Kind:
		return "int64"
	case protoreflect.Sint64Kind:
		return "int64"
	case protoreflect.Uint64Kind:
		return "uint64"
	case protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "[]byte"
	case protoreflect.EnumKind:
		if field.Enum != nil {
			return g.QualifiedGoIdent(field.Enum.GoIdent)
		}
		return "int32" // Fallback for unknown enum types
	case protoreflect.MessageKind:
		if field.Message != nil {
			return g.QualifiedGoIdent(field.Message.GoIdent)
		}
		return "any" // Fallback for unknown message types
	default:
		return "any" // Fallback for unknown types
	}
}
