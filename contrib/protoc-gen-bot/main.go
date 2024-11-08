package main

import (
	"flag"
	"fmt"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

var (
	showVersion = flag.Bool("version", false, "print the version and exit")

	botPackage           = flag.String("bot_package", "github.com/tbxark/sphere/pkg/telegram", "bot package")
	requestModel         = flag.String("request_model", "Update", "request model")
	responseModel        = flag.String("response_model", "Message", "response model")
	extraDataModel       = flag.String("extra_data_model", "MethodExtraData", "extra data model")
	extraDataConstructor = flag.String("extra_data_constructor", "NewMethodExtraData", "extra data constructor")
)

type Config struct {
	botPackage       protogen.GoImportPath
	requestType      string
	responseType     string
	extraType        string
	extraConstructor string
}

func main() {
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-sphere %v\n", "0.0.1")
		return
	}
	cfg := Config{
		botPackage:       protogen.GoImportPath(*botPackage),
		requestType:      *requestModel,
		responseType:     *responseModel,
		extraType:        *extraDataModel,
		extraConstructor: *extraDataConstructor,
	}
	protogen.Options{
		ParamFunc: flag.CommandLine.Set,
	}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			generateFile(gen, f, &cfg)
		}
		return nil
	})
}
