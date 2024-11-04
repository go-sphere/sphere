package main

import (
	"entgo.io/contrib/entproto"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"flag"
	"log"
	"path/filepath"
	"reflect"
	"sort"
)

func main() {
	var (
		schemaPath        = flag.String("path", "./internal/pkg/database/ent/schema", "path to schema directory")
		protoDir          = flag.String("proto", "./proto", "path to proto directory")
		ignoreOptional    = flag.Bool("ignore-optional", true, "ignore optional keyword")
		autoAddAnnotation = flag.Bool("auto-annotation", true, "auto add annotation to the schema")
		enumUseRawType    = flag.Bool("enum-raw-type", true, "use string for enum")
		help              = flag.Bool("help", false, "show help")
	)
	flag.Parse()
	if *help {
		flag.PrintDefaults()
		return
	}
	RunProtoGen(*schemaPath, *protoDir, *ignoreOptional, *autoAddAnnotation, *enumUseRawType)
}

func RunProtoGen(schemaPath string, protoDir string, ignoreOptional, autoAddAnnotation, enumUseString bool) {
	abs, err := filepath.Abs(schemaPath)
	if err != nil {
		log.Fatalf("entproto: failed getting absolute path: %v", err)
	}
	graph, err := entc.LoadGraph(schemaPath, &gen.Config{
		Target: filepath.Dir(abs),
	})
	if err != nil {
		log.Fatalf("entproto: failed loading ent graph: %v", err)
	}
	if autoAddAnnotation {
		for i := 0; i < len(graph.Nodes); i++ {
			node := graph.Nodes[i]
			if node.Annotations == nil {
				node.Annotations = make(map[string]interface{}, 1)
			}
			if node.Annotations[entproto.MessageAnnotation] == nil {
				// If the node does not have the message annotation, add it.
				node.Annotations[entproto.MessageAnnotation] = entproto.Message()
				fieldID := 1
				if node.ID.Annotations == nil {
					node.ID.Annotations = make(map[string]interface{}, 1)
				}
				node.ID.Annotations[entproto.FieldAnnotation] = entproto.Field(fieldID)
				sort.Slice(node.Fields, func(i, j int) bool {
					if node.Fields[i].Position.MixedIn != node.Fields[j].Position.MixedIn {
						// MixedIn fields should be at the end of the list.
						return !node.Fields[i].Position.MixedIn
					}
					return node.Fields[i].Position.Index < node.Fields[j].Position.Index
				})

				for j := 0; j < len(node.Fields); j++ {
					fd := node.Fields[j]
					if fd.Annotations == nil {
						fd.Annotations = make(map[string]interface{}, 1)
					}
					fieldID++
					fd.Annotations[entproto.FieldAnnotation] = entproto.Field(fieldID)
					if fd.IsEnum() {
						if enumUseString {
							if fd.HasGoType() && fd.Type.RType != nil {
								fd.Type.Type = reflectKind2FieldType(fd.Type.RType.Kind)
							} else {
								fd.Type.Type = field.TypeString
							}
						} else {
							enums := make(map[string]int32, len(fd.Enums))
							for index, enum := range fd.Enums {
								enums[enum.Value] = int32(index)
							}
							fd.Annotations[entproto.EnumAnnotation] = entproto.Enum(enums)
						}
					}
					if fd.Optional && ignoreOptional {
						fd.Optional = false
					}
				}
			}
		}
	}
	extension, err := entproto.NewExtension(
		entproto.EnableOptional(),
		entproto.WithProtoDir(protoDir),
		entproto.SkipGenFile(),
	)
	if err != nil {
		log.Fatalf("entproto: failed creating entproto extension: %v", err)
	}
	err = extension.Generate(graph)
	if err != nil {
		log.Fatalf("entproto: failed generating protos: %s", err)
	}
}

func reflectKind2FieldType(kind reflect.Kind) field.Type {
	switch kind {
	case reflect.Bool:
		return field.TypeBool
	case reflect.Int:
		return field.TypeInt
	case reflect.Int8:
		return field.TypeInt8
	case reflect.Int16:
		return field.TypeInt16
	case reflect.Int32:
		return field.TypeInt32
	case reflect.Int64:
		return field.TypeInt64
	case reflect.Uint:
		return field.TypeUint
	case reflect.Uint8:
		return field.TypeUint8
	case reflect.Uint16:
		return field.TypeUint16
	case reflect.Uint32:
		return field.TypeUint32
	case reflect.Uint64:
		return field.TypeUint64
	case reflect.Float32:
		return field.TypeFloat32
	case reflect.Float64:
		return field.TypeFloat64
	case reflect.String:
		return field.TypeString
	case reflect.Slice:
		return field.TypeBytes
	case reflect.Struct:
		return field.TypeJSON
	default:
		return field.TypeOther
	}
}
