package route

import (
	"google.golang.org/protobuf/compiler/protogen"
)

type Config struct {
	OptionsKey   string
	TemplateFile string

	RequestType      protogen.GoIdent
	ResponseType     protogen.GoIdent
	ExtraType        protogen.GoIdent
	ExtraConstructor protogen.GoIdent
}
