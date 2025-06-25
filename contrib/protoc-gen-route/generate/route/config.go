package route

import "github.com/TBXark/sphere/contrib/protoc-gen-route/generate/goident"

type Config struct {
	OptionsKey   string
	FileSuffix   string
	TemplateFile string

	RequestType      *goident.GoIdent
	ResponseType     *goident.GoIdent
	ExtraType        *goident.GoIdent
	ExtraConstructor *goident.GoIdent
}
