package http

import (
	"github.com/TBXark/sphere/cmd/protoc-gen-sphere/generate/template"
	"github.com/TBXark/sphere/internal/protogo"
	"google.golang.org/protobuf/compiler/protogen"
)

type Config struct {
	Omitempty       bool
	OmitemptyPrefix string
	SwaggerAuth     string
	TemplateFile    string

	RouterType    *protogo.GoIdent
	ContextType   *protogo.GoIdent
	ErrorRespType *protogo.GoIdent
	DataRespType  *protogo.GoIdent

	ServerHandlerFunc *protogo.GoIdent
	ParseJsonFunc     *protogo.GoIdent
	ParseUriFunc      *protogo.GoIdent
	ParseFormFunc     *protogo.GoIdent
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
