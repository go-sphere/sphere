package main

import (
	"flag"
	"github.com/tbxark/sphere/contrib/ent-gen-proto/entgenproto"
	"strings"
)

func main() {
	var (
		schemaPath = flag.String("path", "./schema", "path to schema directory")
		protoDir   = flag.String("proto", "./proto", "path to proto directory")

		timeProtoType        = flag.String("time_proto_type", "int64", "use proto type for time.Time, one of int64, string, google.protobuf.Timestamp")
		uuidProtoType        = flag.String("uuid_proto_type", "string", "use proto type for uuid.UUID, one of string, bytes")
		unsupportedProtoType = flag.String("unsupported_proto_type", "google.protobuf.Any", "use proto type for unsupported types, one of google.protobuf.Any, google.protobuf.Struct, bytes")

		allFieldsRequired = flag.Bool("all_fields_required", true, "ignore optional, use zero value instead")
		autoAddAnnotation = flag.Bool("auto_annotation", true, "auto add annotation to the schema")
		enumUseRawType    = flag.Bool("enum_raw_type", true, "use string for enum")

		importProto = flag.String("import_proto", "google/protobuf/any.proto,google.protobuf,Any;", "import proto, format: path1,package1,type1,type2;path2,package2,type3,type4;")

		help = flag.Bool("help", false, "show help")
	)
	flag.Parse()
	if *help {
		flag.PrintDefaults()
		return
	}
	options := entgenproto.Options{
		SchemaPath: *schemaPath,
		ProtoDir:   *protoDir,

		TimeProtoType:        *timeProtoType,
		UUIDProtoType:        *uuidProtoType,
		UnsupportedProtoType: *unsupportedProtoType,

		AllFieldsRequired: *allFieldsRequired,
		AutoAddAnnotation: *autoAddAnnotation,
		EnumUseRawType:    *enumUseRawType,

		ProtoPackages: parseProtoPackages(*importProto),
	}
	entgenproto.Generate(&options)
}

func parseProtoPackages(raw string) []entgenproto.ProtoPackage {
	res := make([]entgenproto.ProtoPackage, 0)
	for _, pkg := range strings.Split(raw, ";") {
		parts := strings.Split(pkg, ",")
		if len(parts) < 3 {
			continue
		}
		res = append(res, entgenproto.ProtoPackage{
			Path:  parts[0],
			Pkg:   parts[1],
			Types: parts[2:],
		})
	}
	return res
}
