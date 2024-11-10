package main

import (
	"flag"
	"github.com/tbxark/sphere/contrib/ent-gen-proto/entgenproto"
)

func main() {
	var (
		schemaPath            = flag.String("path", "./schema", "path to schema directory")
		protoDir              = flag.String("proto", "./proto", "path to proto directory")
		ignoreOptional        = flag.Bool("ignore_optional", true, "ignore optional, use zero value instead")
		ignoreUnsupportedJson = flag.Bool("ignore_unsupported_json", true, "ignore unsupported json type")
		ignoreUnsupportedType = flag.Bool("ignore_unsupported_type", true, "ignore unsupported type")
		autoAddAnnotation     = flag.Bool("auto_annotation", true, "auto add annotation to the schema")
		enumUseRawType        = flag.Bool("enum_raw_type", true, "use string for enum")
		timeUseProtoType      = flag.String("time_proto_type", "google.protobuf.Timestamp", "use proto type for time.Time, one of int64, string, google.protobuf.Timestamp")
		help                  = flag.Bool("help", false, "show help")
	)
	flag.Parse()
	if *help {
		flag.PrintDefaults()
		return
	}
	options := entgenproto.Options{
		SchemaPath:            *schemaPath,
		ProtoDir:              *protoDir,
		IgnoreOptional:        *ignoreOptional,
		IgnoreUnsupportedJson: *ignoreUnsupportedJson,
		IgnoreUnsupportedType: *ignoreUnsupportedType,
		AutoAddAnnotation:     *autoAddAnnotation,
		EnumUseRawType:        *enumUseRawType,
		TimeUseProtoType:      *timeUseProtoType,
	}
	entgenproto.Generate(&options)
}
