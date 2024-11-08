package main

import (
	"flag"
	"fmt"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

var (
	showVersion = flag.Bool("version", false, "print the version and exit")

	clientPackage = flag.String("bot_package", "github.com/go-telegram/bot", "bot package")
	clientModel   = flag.String("bot_model", "Bot", "bot model")

	updatePackage = flag.String("update_package", "github.com/go-telegram/bot/models", "update package")
	updateModel   = flag.String("update_model", "Update", "update model")

	messagePackage = flag.String("message_package", "github.com/tbxark/sphere/pkg/telegram", "message package")
	messageModel   = flag.String("message_model", "Message", "message model")

	extraDataPackage     = flag.String("extra_data_package", "github.com/tbxark/sphere/pkg/telegram", "extra data package")
	extraDataModel       = flag.String("extra_data_model", "MethodExtraData", "extra data model")
	extraDataConstructor = flag.String("extra_data_constructor", "NewMethodExtraData", "extra data constructor")
)

type Package struct {
	pkg   protogen.GoImportPath
	ident string
}

type Config struct {
	client           Package
	update           Package
	message          Package
	extra            Package
	extraConstructor Package
}

func main() {
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-sphere %v\n", "0.0.1")
		return
	}
	cfg := Config{
		update: Package{
			pkg:   protogen.GoImportPath(*updatePackage),
			ident: *updateModel,
		},
		message: Package{
			pkg:   protogen.GoImportPath(*messagePackage),
			ident: *messageModel,
		},
		client: Package{
			pkg:   protogen.GoImportPath(*clientPackage),
			ident: *clientModel,
		},
		extra: Package{
			pkg:   protogen.GoImportPath(*extraDataPackage),
			ident: *extraDataModel,
		},
		extraConstructor: Package{
			pkg:   protogen.GoImportPath(*extraDataPackage),
			ident: *extraDataConstructor,
		},
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
