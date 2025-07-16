package main

import (
	"flag"
	"fmt"

	"github.com/TBXark/sphere/cmd/protoc-gen-sphere-binding/generate/binding"
	"google.golang.org/protobuf/compiler/protogen"
)

var (
	showVersion = flag.Bool("version", false, "print the version and exit")

	out = flag.String("out", "gen", "output directory for generated files")
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
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			binding.GenerateFile(gen, f, *out)
		}
		return nil
	})
}
