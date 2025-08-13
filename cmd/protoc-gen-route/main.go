package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/TBXark/sphere/cmd/protoc-gen-route/generate/route"
	"github.com/TBXark/sphere/cmd/protoc-gen-route/generate/template"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

var (
	showVersion = flag.Bool("version", false, "print the version and exit")

	optionsKey   = flag.String("options_key", "route", "options key in proto")
	templateFile = flag.String("template_file", "", "template file, if not set, use default template")

	requestModel   = flag.String("request_model", "", "request model")
	responseModel  = flag.String("response_model", "", "response model")
	extraDataModel = flag.String("extra_data_model", "", "extra data model")

	extraDataConstructor = flag.String("extra_data_constructor", "", "extra data constructor, and return a pointer of extra data")
)

func main() {
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-route %v\n", "0.0.1")
		return
	}
	protogen.Options{
		ParamFunc: flag.CommandLine.Set,
	}.Run(func(gen *protogen.Plugin) error {
		conf, err := extractConfig()
		if err != nil {
			return err
		}
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		err = template.ReplaceTemplateIfNeed(conf.TemplateFile)
		if err != nil {
			return err
		}
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			_, gErr := route.GenerateFile(gen, f, conf)
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

func extractConfig() (*route.Config, error) {
	_requestModel, err := parseGoIdent(*requestModel)
	if err != nil {
		return nil, err
	}
	_responseModel, err := parseGoIdent(*responseModel)
	if err != nil {
		return nil, err
	}

	conf := &route.Config{
		OptionsKey:   *optionsKey,
		TemplateFile: *templateFile,

		RequestType:  _requestModel,
		ResponseType: _responseModel,
	}

	if *extraDataModel == "" {
		return conf, nil
	}

	_extraDataModel, err := parseGoIdent(*extraDataModel)
	if err != nil {
		return nil, err
	}
	_extraDataConstructor, err := parseGoIdent(*extraDataConstructor)
	if err != nil {
		return nil, err
	}
	conf.ExtraType = _extraDataModel
	conf.ExtraConstructor = _extraDataConstructor

	return conf, nil
}
