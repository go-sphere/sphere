package main

import (
	"flag"
	"fmt"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
	"strings"
)

var (
	showVersion = flag.Bool("version", false, "print the version and exit")

	optionsKey           = flag.String("options_key", "route", "options key in proto")
	genFileSuffix        = flag.String("gen_file_suffix", "_route.pb.go", "generated file suffix")
	requestModel         = flag.String("request_model", "", "request model")
	responseModel        = flag.String("response_model", "", "response model")
	extraDataModel       = flag.String("extra_data_model", "", "extra data model")
	extraDataConstructor = flag.String("extra_data_constructor", "", "extra data constructor, and return a pointer of extra data")
	templateFile         = flag.String("template_file", "", "template file, if not set, use default template")
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
	cfg := Config{
		optionsKey:    *optionsKey,
		genFileSuffix: *genFileSuffix,
		templateFile:  *templateFile,

		requestType:      NewGoIdent(*requestModel),
		responseType:     NewGoIdent(*responseModel),
		extraType:        NewGoIdent(*extraDataModel),
		extraConstructor: NewGoIdent(*extraDataConstructor, identIsFunc()),
	}
	if cfg.requestType == nil {
		return nil, fmt.Errorf("flag request_model must be set")
	}
	if cfg.responseType == nil {
		return nil, fmt.Errorf("flag response_model must be set")
	}
	if cfg.extraType != nil && cfg.extraConstructor == nil {
		return nil, fmt.Errorf("flag extra_data_constructor must be set if extra_data_model is set")
	}
	return &cfg, nil
}

type GoIdent struct {
	pkg    protogen.GoImportPath
	ident  string
	isFunc bool
}

func (g GoIdent) GoIdent() protogen.GoIdent {
	return g.pkg.Ident(g.ident)
}

type Config struct {
	optionsKey    string
	genFileSuffix string
	templateFile  string

	requestType      *GoIdent
	responseType     *GoIdent
	extraType        *GoIdent
	extraConstructor *GoIdent
}

func NewGoIdent(s string, options ...func(*GoIdent)) *GoIdent {
	parts := strings.Split(s, ";")
	if len(parts) != 2 {
		return nil
	}
	ident := &GoIdent{
		pkg:   protogen.GoImportPath(parts[0]),
		ident: parts[1],
	}
	for _, opt := range options {
		opt(ident)
	}
	return ident
}

func identIsFunc() func(*GoIdent) {
	return func(g *GoIdent) {
		g.isFunc = true
	}
}
