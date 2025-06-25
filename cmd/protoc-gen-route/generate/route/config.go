package route

import "github.com/TBXark/sphere/internal/protogo"

type Config struct {
	OptionsKey   string
	FileSuffix   string
	TemplateFile string

	RequestType      *protogo.GoIdent
	ResponseType     *protogo.GoIdent
	ExtraType        *protogo.GoIdent
	ExtraConstructor *protogo.GoIdent
}
