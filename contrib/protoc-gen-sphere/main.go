package main

import (
	"flag"
	"fmt"

	"github.com/TBXark/sphere/contrib/protoc-gen-sphere/generate/goindent"
	"github.com/TBXark/sphere/contrib/protoc-gen-sphere/generate/http"
	"github.com/TBXark/sphere/contrib/protoc-gen-sphere/generate/template"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

const (
	swaggerAuthComment = "// @Param Authorization header string false \"Bearer token\""
	defaultGinPackage  = "github.com/gin-gonic/gin"
	defaultGinxPackage = "github.com/TBXark/sphere/server/ginx"
)

var (
	showVersion = flag.Bool("version", false, "print the version and exit")

	omitempty       = flag.Bool("omitempty", true, "omit if google.api is empty")
	omitemptyPrefix = flag.String("omitempty_prefix", "", "omit if google.api is empty")

	templateFile      = flag.String("template_file", "", "template file, if not set, use default template")
	swaggerAuthHeader = flag.String("swagger_auth_header", swaggerAuthComment, "swagger auth header")

	routerType    = flag.String("router_type", defaultGinPackage+";IRouter", "router type")
	contextType   = flag.String("context_type", defaultGinPackage+";Context", "context type")
	dataRespType  = flag.String("data_resp_type", defaultGinxPackage+";DataResponse", "data response type, must support generic")
	errorRespType = flag.String("error_resp_type", defaultGinxPackage+";ErrorResponse", "error response type")

	serverHandlerFunc = flag.String("server_handler_func", defaultGinxPackage+";WithJson", "server handler func")
	parseJsonFunc     = flag.String("parse_json_func", defaultGinxPackage+";ShouldBindJSON", "parse json func")
	parseUriFunc      = flag.String("parse_uri_func", defaultGinxPackage+";ShouldBindUri", "parse uri func")
	parseFormFunc     = flag.String("parse_form_func", defaultGinxPackage+";ShouldBindQuery", "parse form func")
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
		conf := &http.Config{
			Omitempty:       *omitempty,
			OmitemptyPrefix: *omitemptyPrefix,

			SwaggerAuth:  *swaggerAuthHeader,
			TemplateFile: *templateFile,

			RouterType:    goindent.NewGoIdent(*routerType),
			ContextType:   goindent.NewGoIdent(*contextType),
			ErrorRespType: goindent.NewGoIdent(*errorRespType),
			DataRespType:  goindent.NewGoIdent(*dataRespType, goindent.GenericCount(1)),

			ServerHandlerFunc: goindent.NewGoIdent(*serverHandlerFunc, goindent.IsFunc()),
			ParseJsonFunc:     goindent.NewGoIdent(*parseJsonFunc, goindent.IsFunc()),
			ParseUriFunc:      goindent.NewGoIdent(*parseUriFunc, goindent.IsFunc()),
			ParseFormFunc:     goindent.NewGoIdent(*parseFormFunc, goindent.IsFunc()),
		}
		template.ReplaceTemplateIfNeed(conf.TemplateFile)
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			http.GenerateFile(gen, f, conf)
		}
		return nil
	})
}
