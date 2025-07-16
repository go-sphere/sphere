package parser

import (
	"github.com/TBXark/sphere/proto/binding/sphere/binding"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
)

func CheckBindingLocation(message *protogen.Message, field *protogen.Field, location binding.BindingLocation) bool {
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
