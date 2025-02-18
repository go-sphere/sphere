package gen

import (
	"fmt"
	"github.com/jhump/protoreflect/desc/builder"
	"github.com/jhump/protoreflect/desc/protoprint"
	"github.com/tidwall/gjson"
	"strings"
	"unicode"
)

func Generate(packageName, rootMessage string, rawJson []byte) (string, error) {

	rootMsg := builder.NewMessage(rootMessage)

	parseJSON("", gjson.ParseBytes(rawJson), rootMsg, make(map[string]*builder.MessageBuilder))

	fileBuilder := builder.NewFile(packageName).
		SetPackageName(packageName).
		AddMessage(rootMsg)

	fileBuilder.SetProto3(true)

	printer := protoprint.Printer{
		Indent:  "  ",
		Compact: true,
	}

	fd, err := fileBuilder.Build()
	if err != nil {
		return "", err
	}

	return printer.PrintProtoToString(fd)
}

func parseJSON(prefix string, result gjson.Result, msgBuilder *builder.MessageBuilder, messageCache map[string]*builder.MessageBuilder) {
	if !result.IsObject() {
		return
	}

	result.ForEach(func(key, value gjson.Result) bool {
		fieldName := toSnakeCase(key.String())

		switch {
		case value.IsObject():
			nestedMsgName := strings.Title(key.String())
			var nestedMsg *builder.MessageBuilder

			if cached, ok := messageCache[nestedMsgName]; ok {
				nestedMsg = cached
			} else {
				nestedMsg = builder.NewMessage(nestedMsgName)
				messageCache[nestedMsgName] = nestedMsg
				parseJSON(prefix+key.String()+".", value, nestedMsg, messageCache)
				msgBuilder.AddNestedMessage(nestedMsg)
			}

			msgBuilder.AddField(
				builder.NewField(fieldName, builder.FieldTypeMessage(nestedMsg)),
			)

		case value.IsArray():
			if len(value.Array()) > 0 {
				firstElement := value.Array()[0]
				if firstElement.IsObject() {

					nestedMsgName := strings.Title(strings.TrimSuffix(key.String(), "s"))
					var nestedMsg *builder.MessageBuilder

					if cached, ok := messageCache[nestedMsgName]; ok {
						nestedMsg = cached
					} else {
						nestedMsg = builder.NewMessage(nestedMsgName)
						messageCache[nestedMsgName] = nestedMsg
						parseJSON(prefix+key.String()+".", firstElement, nestedMsg, messageCache)
						msgBuilder.AddNestedMessage(nestedMsg)
					}

					msgBuilder.AddField(
						builder.NewField(fieldName, builder.FieldTypeMessage(nestedMsg)).
							SetRepeated(),
					)
				} else {
					msgBuilder.AddField(
						builder.NewField(fieldName, getProtoType(firstElement)).
							SetRepeated(),
					)
				}
			}

		default:
			msgBuilder.AddField(
				builder.NewField(fieldName, getProtoType(value)),
			)
		}
		return true
	})
}

func getProtoType(value gjson.Result) *builder.FieldType {
	switch {
	case gjson.String == value.Type:
		return builder.FieldTypeString()
	case gjson.Number == value.Type:
		if value.Raw == fmt.Sprintf("%.0f", value.Float()) {
			return builder.FieldTypeInt64()
		}
		return builder.FieldTypeDouble()
	case value.IsBool():
		return builder.FieldTypeBool()
	default:
		return builder.FieldTypeString()
	}
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}
