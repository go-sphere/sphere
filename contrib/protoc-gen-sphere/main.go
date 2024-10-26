package main

import (
	"flag"
	"fmt"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

const (
	swaggerAuthComment = "// @Param Authorization header string false \"Bearer token\""
)

var (
	showVersion       = flag.Bool("version", false, "print the version and exit")
	omitempty         = flag.Bool("omitempty", true, "omit if google.api is empty")
	omitemptyPrefix   = flag.String("omitempty_prefix", "", "omit if google.api is empty")
	swaggerAuthHeader = flag.String("swagger_auth_header", swaggerAuthComment, "swagger auth header")
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
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			generateFile(gen, f, *omitempty, *omitemptyPrefix, *swaggerAuthHeader)
		}
		return nil
	})
}
