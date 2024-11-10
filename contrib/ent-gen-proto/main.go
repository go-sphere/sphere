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
	_ "unsafe"
)

//go:linkname generate entgo.io/contrib/entproto.(*Extension).generate
func generate(extension *entproto.Extension, g *gen.Graph) error

func main() {
	var (
		schemaPath        = flag.String("path", "./schema", "path to schema directory")
		protoDir          = flag.String("proto", "./proto", "path to proto directory")
		ignoreOptional    = flag.Bool("ignore_optional", true, "ignore optional, use zero value instead")
		autoAddAnnotation = flag.Bool("auto_annotation", true, "auto add annotation to the schema")
		enumUseRawType    = flag.Bool("enum_raw_type", true, "use string for enum")
		timeUseProtoType  = flag.String("time_proto_type", "google.protobuf.Timestamp", "use proto type for time.Time, one of int64, string, google.protobuf.Timestamp")
		help              = flag.Bool("help", false, "show help")
	)
	flag.Parse()
	if *help {
		flag.PrintDefaults()
		return
	}
	runProtoGen(*schemaPath, *protoDir, *ignoreOptional, *autoAddAnnotation, *enumUseRawType, *timeUseProtoType)
}

func runProtoGen(schemaPath string, protoDir string, ignoreOptional, autoAddAnnotation, enumUseRawType bool, timeUseProtoType string) {
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
			addAnnotationForNode(graph.Nodes[i], enumUseRawType, ignoreOptional, timeUseProtoType)
		}
	}
	extension, err := entproto.NewExtension(
		entproto.WithProtoDir(protoDir),
		entproto.SkipGenFile(),
	)
	if err != nil {
		log.Fatalf("entproto: failed creating entproto extension: %v", err)
	}
	err = generate(extension, graph)
	if err != nil {
		log.Fatalf("entproto: failed generating protos: %s", err)
	}
}

func addAnnotationForNode(node *gen.Type, enumUseRawType bool, ignoreOptional bool, timeUseProtoType string) {
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
			if fd.IsEnum() {
				fixEnumType(fd, enumUseRawType)
			}
			fd.Type.Type = fixFieldType(fd, timeUseProtoType)
			fd.Annotations[entproto.FieldAnnotation] = entproto.Field(fieldID)
			if fd.Optional && ignoreOptional {
				fd.Optional = false
			}
		}
	}
}

func fixEnumType(fd *gen.Field, enumUseRawType bool) {
	if enumUseRawType {
		if fd.HasGoType() {
			fd.Type.Type = reflectKind2FieldType[fd.Type.RType.Kind]
		} else {
			fd.Type.Type = field.TypeString
		}
	} else {
		enums := make(map[string]int32, len(fd.Enums))
		for index, enum := range fd.Enums {
			enums[enum.Value] = int32(index) + 1
		}
		fd.Annotations[entproto.EnumAnnotation] = entproto.Enum(enums, entproto.OmitFieldPrefix())
	}
}

func fixFieldType(fd *gen.Field, timeType string) field.Type {
	switch fd.Type.Type {
	case field.TypeOther, field.TypeInvalid:
		return field.TypeBytes // JSON and Other types are mapped to bytes.
	case field.TypeJSON:
		if fd.HasGoType() {
			switch fd.Type.RType.Kind {
			case reflect.Slice:
				// 只支持基础类型的切片
				if _, ok := buildInTypeSlice[fd.Type.RType.Ident]; ok {
					return field.TypeJSON
				}
				return field.TypeJSON
			default:
				return field.TypeBytes
			}
		}
		return field.TypeBytes
	case field.TypeUUID:
		return field.TypeString
	case field.TypeTime:
		switch timeType {
		case "int64":
			return field.TypeInt64
		case "string":
			return field.TypeString
		default:
			return field.TypeTime
		}
	default:
		return fd.Type.Type
	}
}

var reflectKind2FieldType = map[reflect.Kind]field.Type{
	reflect.Bool:          field.TypeBool,
	reflect.Int:           field.TypeInt,
	reflect.Int8:          field.TypeInt8,
	reflect.Int16:         field.TypeInt16,
	reflect.Int32:         field.TypeInt32,
	reflect.Int64:         field.TypeInt64,
	reflect.Uint:          field.TypeUint,
	reflect.Uint8:         field.TypeUint8,
	reflect.Uint16:        field.TypeUint16,
	reflect.Uint32:        field.TypeUint32,
	reflect.Uint64:        field.TypeUint64,
	reflect.Uintptr:       field.TypeUint,
	reflect.Float32:       field.TypeFloat32,
	reflect.Float64:       field.TypeFloat64,
	reflect.Complex64:     field.TypeOther,
	reflect.Complex128:    field.TypeOther,
	reflect.Array:         field.TypeJSON,
	reflect.Chan:          field.TypeOther,
	reflect.Func:          field.TypeOther,
	reflect.Interface:     field.TypeJSON,
	reflect.Map:           field.TypeJSON,
	reflect.Pointer:       field.TypeJSON,
	reflect.Slice:         field.TypeJSON,
	reflect.String:        field.TypeString,
	reflect.Struct:        field.TypeJSON,
	reflect.UnsafePointer: field.TypeOther,
}

var buildInTypeSlice = map[string]struct{}{
	"[]int":     {},
	"[]int8":    {},
	"[]int16":   {},
	"[]int32":   {},
	"[]int64":   {},
	"[]uint":    {},
	"[]uint8":   {},
	"[]uint16":  {},
	"[]uint32":  {},
	"[]uint64":  {},
	"[]float32": {},
	"[]float64": {},
	"[]string":  {},
	"[]bool":    {},
}
