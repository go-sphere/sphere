package main

import (
	"flag"
	"fmt"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
	"strings"
)

const (
	swaggerAuthComment = "// @Param Authorization header string false \"Bearer token\""
)

var (
	showVersion     = flag.Bool("version", false, "print the version and exit")
	omitempty       = flag.Bool("omitempty", true, "omit if google.api is empty")
	omitemptyPrefix = flag.String("omitempty_prefix", "", "omit if google.api is empty")

	templateFile = flag.String("template_file", "", "template file, if not set, use default template")

	swaggerAuthHeader = flag.String("swagger_auth_header", swaggerAuthComment, "swagger auth header")

	routerType    = flag.String("router_type", "github.com/gin-gonic/gin;IRouter", "router type")
	contextType   = flag.String("context_type", "github.com/gin-gonic/gin;Context", "context type")
	dataRespType  = flag.String("data_resp_type", "github.com/TBXark/sphere/pkg/server/ginx;DataResponse", "data response type, must support generic")
	errorRespType = flag.String("error_resp_type", "github.com/TBXark/sphere/pkg/server/ginx;ErrorResponse", "error response type")

	serverHandlerFunc = flag.String("server_handler_func", "github.com/TBXark/sphere/pkg/server/ginx;WithJson", "server handler func")
	parseJsonFunc     = flag.String("parse_json_func", "github.com/TBXark/sphere/pkg/server/ginx;ShouldBindJSON", "parse json func")
	parseUriFunc      = flag.String("parse_uri_func", "github.com/TBXark/sphere/pkg/server/ginx;ShouldBindUri", "parse uri func")
	parseFormFunc     = flag.String("parse_form_func", "github.com/TBXark/sphere/pkg/server/ginx;ShouldBindQuery", "parse form func")
)

func main() {
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-sphere %v\n", "0.0.1")
		return
	}
	protogen.Options{
		ParamFunc: flag.CommandLine.Set,
	}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		conf := &Config{
			omitempty:       *omitempty,
			omitemptyPrefix: *omitemptyPrefix,
			swaggerAuth:     *swaggerAuthHeader,
			templateFile:    *templateFile,

			routerType:    NewGoIdent(*routerType),
			contextType:   NewGoIdent(*contextType),
			errorRespType: NewGoIdent(*errorRespType),
			dataRespType:  NewGoIdent(*dataRespType, identGenericCount(1)),

			serverHandlerFunc: NewGoIdent(*serverHandlerFunc, identIsFunc()),
			parseJsonFunc:     NewGoIdent(*parseJsonFunc, identIsFunc()),
			parseUriFunc:      NewGoIdent(*parseUriFunc, identIsFunc()),
			parseFormFunc:     NewGoIdent(*parseFormFunc, identIsFunc()),
		}
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			generateFile(gen, f, conf)
		}
		return nil
	})
}

type GoIdent struct {
	pkg          protogen.GoImportPath
	ident        string
	isFunc       bool
	genericCount int
}

func (g GoIdent) GoIdent() protogen.GoIdent {
	return g.pkg.Ident(g.ident)
}

func NewGoIdent(s string, options ...func(*GoIdent)) *GoIdent {
	parts := strings.Split(s, ";")
	if len(parts) != 2 {
		return nil
	}
	i := &GoIdent{
		pkg:   protogen.GoImportPath(parts[0]),
		ident: parts[1],
	}
	for _, option := range options {
		option(i)
	}
	return i
}

func identIsFunc() func(*GoIdent) {
	return func(g *GoIdent) {
		g.isFunc = true
	}
}

func identGenericCount(count int) func(*GoIdent) {
	return func(g *GoIdent) {
		g.genericCount = count
	}
}

type Config struct {
	omitempty       bool
	omitemptyPrefix string
	swaggerAuth     string
	templateFile    string

	routerType    *GoIdent
	contextType   *GoIdent
	errorRespType *GoIdent
	dataRespType  *GoIdent

	serverHandlerFunc *GoIdent
	parseJsonFunc     *GoIdent
	parseUriFunc      *GoIdent
	parseFormFunc     *GoIdent
}
