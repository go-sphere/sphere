package http

import (
	"github.com/TBXark/sphere/cmd/protoc-gen-sphere/generate/template"
	"google.golang.org/protobuf/compiler/protogen"
)

type Config struct {
	Omitempty       bool
	OmitemptyPrefix string
	SwaggerAuth     string
	TemplateFile    string

	RouterType    protogen.GoIdent
	ContextType   protogen.GoIdent
	ErrorRespType protogen.GoIdent
	DataRespType  protogen.GoIdent

	ServerHandlerFunc protogen.GoIdent
	ParseJsonFunc     protogen.GoIdent
	ParseUriFunc      protogen.GoIdent
	ParseFormFunc     protogen.GoIdent
}

type GenConfig struct {
	omitempty       bool
	omitemptyPrefix string
	swaggerAuth     string
	packageDesc     *template.PackageDesc
}

func NewGenConf(g *protogen.GeneratedFile, conf *Config) *GenConfig {
	pkgDesc := &template.PackageDesc{
		RouterType:  g.QualifiedGoIdent(conf.RouterType),
		ContextType: g.QualifiedGoIdent(conf.ContextType),

		ErrorResponseType: g.QualifiedGoIdent(conf.ErrorRespType),
		DataResponseType:  g.QualifiedGoIdent(conf.DataRespType),

		ParseJsonFunc:            g.QualifiedGoIdent(conf.ParseJsonFunc),
		ParseUriFunc:             g.QualifiedGoIdent(conf.ParseUriFunc),
		ParseFormFunc:            g.QualifiedGoIdent(conf.ParseFormFunc),
		ServerHandlerWrapperFunc: g.QualifiedGoIdent(conf.ServerHandlerFunc),
	}
	genConf := &GenConfig{
		omitempty:       conf.Omitempty,
		omitemptyPrefix: conf.OmitemptyPrefix,
		swaggerAuth:     conf.SwaggerAuth,
		packageDesc:     pkgDesc,
	}
	return genConf
}
