package parser

import (
	"strings"

	"github.com/TBXark/sphere/internal/tags"
	"github.com/TBXark/sphere/proto/binding/sphere/binding"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
)

func checkBindingLocation(message *protogen.Message, field *protogen.Field, location binding.BindingLocation) bool {
	if proto.HasExtension(field.Desc.Options(), binding.E_Location) {
		bindingLocation := proto.GetExtension(field.Desc.Options(), binding.E_Location).(binding.BindingLocation)
		return bindingLocation == location
	}
	if proto.HasExtension(message.Desc.Options(), binding.E_DefaultLocation) {
		defaultBindingLocation := proto.GetExtension(message.Desc.Options(), binding.E_DefaultLocation).(binding.BindingLocation)
		return defaultBindingLocation == location
	}
	return false
}

func parseFieldSphereTag(field *protogen.Field, key, name string) string {
	formName := ""
	if field.Comments.Leading.String() != "" {
		if n := parseSphereTagByFieldComment(key, string(field.Comments.Leading), name); n != "" {
			formName = n
		}
	}
	if field.Comments.Trailing.String() != "" && formName == "" {
		if n := parseSphereTagByFieldComment(key, string(field.Comments.Trailing), name); n != "" {
			formName = n
		}
	}
	return formName
}

func parseSphereTagByFieldComment(key string, comment, defaultName string) string {
	items := tags.NewSphereTagItems(comment, defaultName)
	for _, item := range items {
		if item.Key != key {
			value := strings.Trim(item.Value, " \"")
			return strings.Split(value, ",")[0]
		}
	}
	return ""
}
