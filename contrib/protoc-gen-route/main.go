package main

import (
	"flag"
	"fmt"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

var (
	showVersion = flag.Bool("version", false, "print the version and exit")

	optionsKey           = flag.String("options_key", "route", "options key in proto")
	genFileSuffix        = flag.String("gen_file_suffix", "_route.pb.go", "generated file suffix")
	routePackage         = flag.String("route_package", "", "route package")
	requestModel         = flag.String("request_model", "", "request model")
	responseModel        = flag.String("response_model", "", "response model")
	extraDataModel       = flag.String("extra_data_model", "", "extra data model")
	extraDataConstructor = flag.String("extra_data_constructor", "", "extra data constructor")
	templateFile         = flag.String("template_file", "", "template file, if not set, use default template")
)

type Config struct {
	optionsKey    string
	genFileSuffix string
	templateFile  string

	routePackage     protogen.GoImportPath
	requestType      string
	responseType     string
	extraType        string
	extraConstructor string
}

func main() {
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-route %v\n", "0.0.1")
		return
	}
	protogen.Options{
		ParamFunc: flag.CommandLine.Set,
	}.Run(func(gen *protogen.Plugin) error {
		cfg, err := genConfig()
		if err != nil {
			return err
		}
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			generateFile(gen, f, cfg)
		}
		return nil
	})
}

func genConfig() (*Config, error) {
	if *routePackage == "" {
		return nil, fmt.Errorf("flag route_package must be set")
	}
	if *requestModel == "" {
		return nil, fmt.Errorf("flag request_model must be set")
	}
	if *responseModel == "" {
		return nil, fmt.Errorf("flag response_model must be set")
	}
	if *extraDataModel != "" && *extraDataConstructor == "" {
		return nil, fmt.Errorf("flag extra_data_constructor must be set if extra_data_model is set")
	}
	cfg := Config{
		optionsKey:    *optionsKey,
		genFileSuffix: *genFileSuffix,
		templateFile:  *templateFile,

		routePackage:     protogen.GoImportPath(*routePackage),
		requestType:      *requestModel,
		responseType:     *responseModel,
		extraType:        *extraDataModel,
		extraConstructor: *extraDataConstructor,
	}
	return &cfg, nil
}
