package cmd

import (
	"github.com/TBXark/sphere/cmd/sphere-cli/internal/entgenproto"
	"github.com/spf13/cobra"
)

var ent2protoCmd = &cobra.Command{
	Use:   "ent2proto",
	Short: "Convert Ent schema to Protobuf definitions",
	Long:  `Convert Ent schema to Protobuf definitions, generating .proto files from Ent schema definitions.`,
}

func init() {
	rootCmd.AddCommand(ent2protoCmd)

	flag := ent2protoCmd.Flags()
	schemaPath := flag.String("path", "./schema", "path to schema directory")
	protoDir := flag.String("proto", "./proto", "path to proto directory")

	timeProtoType := flag.String("time_proto_type", "int64", "use proto type for time.Time, one of int64, string, google.protobuf.Timestamp")
	uuidProtoType := flag.String("uuid_proto_type", "string", "use proto type for uuid.UUID, one of string, bytes")
	unsupportedProtoType := flag.String("unsupported_proto_type", "google.protobuf.Any", "use proto type for unsupported types, one of google.protobuf.Any, google.protobuf.Struct, bytes")

	allFieldsRequired := flag.Bool("all_fields_required", true, "ignore optional, use zero value instead")
	autoAddAnnotation := flag.Bool("auto_annotation", true, "auto add annotation to the schema")
	enumUseRawType := flag.Bool("enum_raw_type", true, "use string for enum")
	skipUnsupported := flag.Bool("skip_unsupported", true, "skip unsupported types, when unsupportedProtoType is not set")

	importProto := flag.String("import_proto", "google/protobuf/any.proto,google.protobuf,Any;", "import proto, format: path1,package1,type1,type2;path2,package2,type3,type4;")

	ent2protoCmd.RunE = func(cmd *cobra.Command, args []string) error {
		options := entgenproto.Options{
			SchemaPath: *schemaPath,
			ProtoDir:   *protoDir,

			TimeProtoType:        *timeProtoType,
			UUIDProtoType:        *uuidProtoType,
			UnsupportedProtoType: *unsupportedProtoType,
			SkipUnsupported:      *skipUnsupported,

			AllFieldsRequired: *allFieldsRequired,
			AutoAddAnnotation: *autoAddAnnotation,
			EnumUseRawType:    *enumUseRawType,

			ProtoPackages: entgenproto.ParseProtoPackages(*importProto),
		}
		entgenproto.Generate(&options)
		return nil
	}
}
