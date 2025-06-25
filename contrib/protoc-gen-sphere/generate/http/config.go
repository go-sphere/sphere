package http

import (
	"github.com/TBXark/sphere/contrib/protoc-gen-sphere/generate/goindent"
	"github.com/TBXark/sphere/contrib/protoc-gen-sphere/generate/template"
	"google.golang.org/protobuf/compiler/protogen"
)

type Config struct {
	Omitempty       bool
	OmitemptyPrefix string
	SwaggerAuth     string
	TemplateFile    string

	RouterType    *goindent.GoIdent
	ContextType   *goindent.GoIdent
	ErrorRespType *goindent.GoIdent
	DataRespType  *goindent.GoIdent

	ServerHandlerFunc *goindent.GoIdent
	ParseJsonFunc     *goindent.GoIdent
	ParseUriFunc      *goindent.GoIdent
	ParseFormFunc     *goindent.GoIdent
}

type GenConfig struct {
	omitempty       bool
	omitemptyPrefix string
	swaggerAuth     string
	packageDesc     *template.PackageDesc
}

func NewGenConf(g *protogen.GeneratedFile, conf *Config) *GenConfig {
	pkgDesc := &template.PackageDesc{
		RouterType:               g.QualifiedGoIdent(conf.RouterType.GoIdent()),
		ContextType:              g.QualifiedGoIdent(conf.ContextType.GoIdent()),
		ErrorResponseType:        g.QualifiedGoIdent(conf.ErrorRespType.GoIdent()),
		DataResponseType:         g.QualifiedGoIdent(conf.DataRespType.GoIdent()),
		ServerHandlerWrapperFunc: g.QualifiedGoIdent(conf.ServerHandlerFunc.GoIdent()),
		ParseJsonFunc:            g.QualifiedGoIdent(conf.ParseJsonFunc.GoIdent()),
		ParseUriFunc:             g.QualifiedGoIdent(conf.ParseUriFunc.GoIdent()),
		ParseFormFunc:            g.QualifiedGoIdent(conf.ParseFormFunc.GoIdent()),
	}
	genConf := &GenConfig{
		omitempty:       conf.Omitempty,
		omitemptyPrefix: conf.OmitemptyPrefix,
		swaggerAuth:     conf.SwaggerAuth,
		packageDesc:     pkgDesc,
	}
	return genConf
}
