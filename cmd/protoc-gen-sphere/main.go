package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/TBXark/sphere/cmd/protoc-gen-sphere/generate/http"
	"github.com/TBXark/sphere/cmd/protoc-gen-sphere/generate/template"
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
	errorRespType = flag.String("error_resp_type", defaultGinxPackage+";ErrorResponse", "error response type")
	dataRespType  = flag.String("data_resp_type", defaultGinxPackage+";DataResponse", "data response type, must support generic")

	parseJsonFunc     = flag.String("parse_json_func", defaultGinxPackage+";ShouldBindJSON", "parse json func")
	parseUriFunc      = flag.String("parse_uri_func", defaultGinxPackage+";ShouldBindUri", "parse uri func")
	parseFormFunc     = flag.String("parse_form_func", defaultGinxPackage+";ShouldBindQuery", "parse form func")
	serverHandlerFunc = flag.String("server_handler_func", defaultGinxPackage+";WithJson", "server handler func, must support generic")
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
		conf, err := extractConfig()
		if err != nil {
			return err
		}
		template.ReplaceTemplateIfNeed(conf.TemplateFile)
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			_, gErr := http.GenerateFile(gen, f, conf)
			if gErr != nil {
				return gErr
			}
		}
		return nil
	})
}

func parseGoIdent(raw string) (protogen.GoIdent, error) {
	parts := strings.Split(raw, ";")
	if len(parts) != 2 {
		return protogen.GoIdent{}, errors.New("invalid GoIdent format, expected 'path;ident'")
	}
	return protogen.GoIdent{
		GoName:       parts[1],
		GoImportPath: protogen.GoImportPath(parts[0]),
	}, nil
}

func extractConfig() (*http.Config, error) {
	_routerType, err := parseGoIdent(*routerType)
	if err != nil {
		return nil, err
	}
	_contextType, err := parseGoIdent(*contextType)
	if err != nil {
		return nil, err
	}
	_errorRespType, err := parseGoIdent(*errorRespType)
	if err != nil {
		return nil, err
	}
	_dataRespType, err := parseGoIdent(*dataRespType)
	if err != nil {
		return nil, err
	}

	_serverHandlerFunc, err := parseGoIdent(*serverHandlerFunc)
	if err != nil {
		return nil, err
	}
	_parseJsonFunc, err := parseGoIdent(*parseJsonFunc)
	if err != nil {
		return nil, err
	}
	_parseUriFunc, err := parseGoIdent(*parseUriFunc)
	if err != nil {
		return nil, err
	}
	_parseFormFunc, err := parseGoIdent(*parseFormFunc)
	if err != nil {
		return nil, err
	}

	conf := &http.Config{
		Omitempty:       *omitempty,
		OmitemptyPrefix: *omitemptyPrefix,

		SwaggerAuth:  *swaggerAuthHeader,
		TemplateFile: *templateFile,

		RouterType:    _routerType,
		ContextType:   _contextType,
		ErrorRespType: _errorRespType,
		DataRespType:  _dataRespType,

		ServerHandlerFunc: _serverHandlerFunc,
		ParseJsonFunc:     _parseJsonFunc,
		ParseUriFunc:      _parseUriFunc,
		ParseFormFunc:     _parseFormFunc,
	}
	return conf, nil
}
