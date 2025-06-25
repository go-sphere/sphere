package entgenproto

import (
	"errors"
	"fmt"
	"log"
	"path"
	"reflect"
	"sort"
	"strings"
	_ "unsafe"

	"entgo.io/contrib/entproto"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"github.com/jhump/protoreflect/desc" //nolint
	"github.com/jhump/protoreflect/desc/protoprint"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/multierr"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Options struct {
	SchemaPath string
	ProtoDir   string

	AllFieldsRequired bool
	AutoAddAnnotation bool
	EnumUseRawType    bool
	SkipUnsupported   bool

	TimeProtoType        string
	UUIDProtoType        string
	UnsupportedProtoType string

	ProtoPackages []ProtoPackage
}

type ProtoPackage struct {
	Path  string
	Pkg   string
	Types []string
}

func Generate(options *Options) {
	injectProtoPackages(options.ProtoPackages)
	graph, err := entc.LoadGraph(options.SchemaPath, &gen.Config{
		Target: options.SchemaPath,
	})
	if err != nil {
		log.Fatalf("entproto: failed loading ent graph: %v", err)
	}
	if options.AutoAddAnnotation {
		for i := 0; i < len(graph.Nodes); i++ {
			addAnnotationForNode(graph.Nodes[i], options)
		}
	}
	err = generate(graph, options)
	if err != nil {
		log.Fatalf("entproto: failed generating protos: %s", err)
	}
}

func generate(g *gen.Graph, options *Options) error {
	entProtoDir := path.Join(g.Target, "proto")
	if options.ProtoDir != "" {
		entProtoDir = options.ProtoDir
	}
	adapter, err := entproto.LoadAdapter(g)
	if err != nil {
		return fmt.Errorf("entproto: failed parsing ent graph: %w", err)
	}
	var errs error
	for _, schema := range g.Schemas {
		name := schema.Name
		_, sErr := adapter.GetFileDescriptor(name)
		if sErr != nil && !errors.Is(sErr, entproto.ErrSchemaSkipped) {
			errs = multierr.Append(errs, sErr)
		}
	}
	if errs != nil {
		return fmt.Errorf("entproto: failed parsing some schemas: %w", errs)
	}
	allDescriptors := make([]*desc.FileDescriptor, 0, len(adapter.AllFileDescriptors()))
	for _, fDesc := range adapter.AllFileDescriptors() {
		fixProto3Optional(g, fDesc)
		allDescriptors = append(allDescriptors, fDesc)
	}
	var printer protoprint.Printer
	if err = printer.PrintProtosToFileSystem(allDescriptors, entProtoDir); err != nil {
		return fmt.Errorf("entproto: failed writing .proto files: %w", err)
	}
	return nil
}

func addAnnotationForNode(node *gen.Type, options *Options) {
	if node.Annotations == nil {
		node.Annotations = make(map[string]interface{}, 1)
	}
	if node.Annotations[entproto.MessageAnnotation] != nil {
		return
	}
	// If the node does not have the message annotation, add it.
	node.Annotations[entproto.MessageAnnotation] = entproto.Message()
	idGenerator := &fieldIDGenerator{exist: extractExistFieldID(node)}
	sort.Slice(node.Fields, func(i, j int) bool {
		if node.Fields[i].Position.MixedIn != node.Fields[j].Position.MixedIn {
			// MixedIn fields should be at the end of the list.
			return !node.Fields[i].Position.MixedIn
		}
		return node.Fields[i].Position.Index < node.Fields[j].Position.Index
	})
	addAnnotationForField(node.ID, idGenerator, options)
	for j := 0; j < len(node.Fields); j++ {
		addAnnotationForField(node.Fields[j], idGenerator, options)
	}
}

func addAnnotationForField(fd *gen.Field, idGenerator *fieldIDGenerator, options *Options) {
	if fd.Annotations == nil {
		fd.Annotations = make(map[string]interface{}, 1)
	}
	if fd.Annotations[entproto.FieldAnnotation] != nil {
		return
	}
	if fd.Annotations[entproto.SkipAnnotation] != nil {
		return
	}
	var fieldOptions []entproto.FieldOption
	switch fd.Type.Type {
	case field.TypeEnum:
		fixEnumType(fd, options.EnumUseRawType)
	case field.TypeJSON:
		if _, ok := entprotoSupportJSONType[fd.Type.RType.Ident]; !ok {
			if options.SkipUnsupported {
				fd.Annotations[entproto.SkipAnnotation] = entproto.Skip()
				return
			} else {
				nt, opts := fixUnsupportedType(options.UnsupportedProtoType)
				fd.Type.Type = nt
				fieldOptions = append(fieldOptions, opts...)
			}
		}
	case field.TypeOther:
		if options.SkipUnsupported {
			fd.Annotations[entproto.SkipAnnotation] = entproto.Skip()
			return
		} else {
			nt, opts := fixUnsupportedType(options.UnsupportedProtoType)
			fd.Type.Type = nt
			fieldOptions = append(fieldOptions, opts...)
		}
	case field.TypeTime:
		switch options.TimeProtoType {
		case "int64":
			fd.Type.Type = field.TypeInt64
		case "string":
			fd.Type.Type = field.TypeString
		default:
			break
		}
	case field.TypeUUID:
		switch options.UUIDProtoType {
		case "string":
			fd.Type.Type = field.TypeString
		default:
			break
		}
	default:
		break
	}
	fd.Annotations[entproto.FieldAnnotation] = entproto.Field(idGenerator.MustNext(), fieldOptions...)
	if fd.Optional {
		fd.Optional = false
		if !options.AllFieldsRequired {
			fd.Annotations[FieldIsProto3Optional] = struct{}{}
		}
	}
}

//

func fixUnsupportedType(unsupportedProtoType string) (field.Type, []entproto.FieldOption) {
	switch unsupportedProtoType {
	case "bytes":
		return field.TypeBytes, nil
	default:
		return field.TypeJSON, []entproto.FieldOption{
			entproto.Type(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
			entproto.TypeName(unsupportedProtoType),
		}
	}
}

func fixEnumType(fd *gen.Field, enumUseRawType bool) {
	if fd.Annotations[entproto.EnumAnnotation] != nil {
		return
	}
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

const FieldIsProto3Optional = "IsProto3Optional"

var (
	isProto3OptionalValue = true
	optionalFieldLabel    = descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
)

func fixProto3Optional(g *gen.Graph, fDesc *desc.FileDescriptor) {
	messageMap := make(map[string]*desc.MessageDescriptor)
	for _, message := range fDesc.GetMessageTypes() {
		messageMap[message.GetName()] = message
	}
	for _, node := range g.Nodes {
		message, ok := messageMap[node.Name]
		if !ok {
			continue
		}
		for _, fd := range node.Fields {
			if fd.Annotations != nil && fd.Annotations[FieldIsProto3Optional] != nil {
				pbFd := message.FindFieldByName(fd.Name)
				if pbFd != nil {
					proto := pbFd.AsFieldDescriptorProto()
					if proto.Label == nil || *proto.Label == descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL {
						proto.Label = &optionalFieldLabel
						proto.Proto3Optional = &isProto3OptionalValue
					}
				}
			}
		}
	}
}

/// Type Mapping

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

// entprotoSupportJSONType
// entgo.io/contrib/entproto.extractJSONDetails
var entprotoSupportJSONType = map[string]struct{}{
	"[]int32":  {},
	"[]int64":  {},
	"[]uint32": {},
	"[]uint64": {},
	"[]string": {},
}

/// ID Generator

type fieldIDGenerator struct {
	current int
	exist   map[int]struct{}
}

func (f *fieldIDGenerator) Next() (int, error) {
	f.current++
	for {
		if _, ok := f.exist[f.current]; ok {
			f.current++
			continue
		}
		if f.current > 536870911 {
			return 0, fmt.Errorf("entproto: field number exceed the maximum value 536870911")
		}
		break
	}
	return f.current, nil
}

func (f *fieldIDGenerator) MustNext() int {
	num, err := f.Next()
	if err != nil {
		panic(err)
	}
	return num
}

func extractExistFieldID(node *gen.Type) map[int]struct{} {
	maxExistNum := 0
	existNums := map[int]struct{}{}
	for _, fd := range node.Fields {
		if fd.Annotations != nil {
			if obj, exist := fd.Annotations[entproto.FieldAnnotation]; exist {
				pbField := struct {
					Number int
				}{}
				err := mapstructure.Decode(obj, &pbField)
				if err != nil {
					log.Fatalf("entproto: failed decoding field annotation: %v", err)
				}
				existNums[pbField.Number] = struct{}{}
				if pbField.Number > maxExistNum {
					maxExistNum = pbField.Number
				}
			}
		}
	}
	return existNums
}

/// Inject

//go:linkname wktsPaths entgo.io/contrib/entproto.wktsPaths
var wktsPaths map[string]string

func injectProtoPackages(pkg []ProtoPackage) {
	wktsPaths["google.protobuf.Any"] = "google/protobuf/any.proto"
	wktsPaths["google.protobuf.Struct"] = "google/protobuf/struct.proto"
	for _, p := range pkg {
		for _, t := range p.Types {
			wktsPaths[p.Pkg+"."+t] = p.Path
		}
	}
}

func ParseProtoPackages(raw string) []ProtoPackage {
	res := make([]ProtoPackage, 0)
	for _, pkg := range strings.Split(raw, ";") {
		parts := strings.Split(pkg, ",")
		if len(parts) < 3 {
			continue
		}
		res = append(res, ProtoPackage{
			Path:  parts[0],
			Pkg:   parts[1],
			Types: parts[2:],
		})
	}
	return res
}
