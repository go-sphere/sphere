package main

import (
	"entgo.io/ent/entc/load"
	"entgo.io/ent/schema/field"
	"golang.org/x/exp/maps"
	"strings"
)

type FieldDesc struct {
	ProtoType string
	Name      string
	Index     int
	Comment   string
	Optional  bool
}

type SchemaDesc struct {
	Name   string
	Fields []*FieldDesc
}

type FileDesc struct {
	Package string
	Imports []string
	Schemas []*SchemaDesc
}

func genProtoType(goType string) string {
	if t, ok := goType2ProtoBuildInTypes[goType]; ok {
		return t
	}
	if t := goTypeArrayRegexp.FindStringSubmatch(goType); len(t) == 2 {
		return "repeated " + genProtoType(t[1])
	}
	if t := goTypeMapRegexp.FindStringSubmatch(goType); len(t) == 3 {
		return "map<" + genProtoType(t[1]) + ", " + genProtoType(t[2]) + ">"
	}
	return goType
}

func genFileDesc(protoPackage *string, spec *load.SchemaSpec) FileDesc {
	fileDesc := FileDesc{
		Package: *protoPackage,
		Schemas: nil,
	}
	imports := map[string]struct{}{}
	for _, s := range spec.Schemas {
		schemaDesc := genSchemaDesc(s)
		fileDesc.Schemas = append(fileDesc.Schemas, &schemaDesc)
	}
	for _, s := range fileDesc.Schemas {
		for _, f := range s.Fields {
			if f.ProtoType == "google.protobuf.Timestamp" {
				imports["google/protobuf/timestamp.proto"] = struct{}{}
			}
			if f.ProtoType == "google.protobuf.Any" {
				imports["google/protobuf/any.proto"] = struct{}{}
			}
		}
	}
	fileDesc.Imports = maps.Keys(imports)

	return fileDesc
}

func genSchemaDesc(s *load.Schema) SchemaDesc {
	schemaDesc := SchemaDesc{
		Name:   s.Name,
		Fields: nil,
	}
	fields := make([]FieldDesc, 0)
	mixFields := make([]FieldDesc, 0)
	for _, f := range s.Fields {
		fieldDesc := genFieldDesc(f)
		if f.Position.MixedIn {
			mixFields = append(mixFields, fieldDesc)
		} else {
			fields = append(fields, fieldDesc)
		}
	}
	for i, f := range fields {
		f.Index = i + 1
		schemaDesc.Fields = append(schemaDesc.Fields, &f)
	}
	for i, f := range mixFields {
		f.Index = i + len(fields) + 1
		schemaDesc.Fields = append(schemaDesc.Fields, &f)
	}
	return schemaDesc
}

func genFieldDesc(f *load.Field) FieldDesc {
	protoType := protoTypeMap[f.Info.Type]
	if protoType == "" {
		protoType = "google.protobuf.Any"
	}
	if f.Info.Type == field.TypeJSON {
		if strings.HasPrefix(f.Info.Ident, "map[") {
			protoType = genProtoType(f.Info.Ident)
		} else if strings.HasPrefix(f.Info.Ident, "[]") {
			protoType = genProtoType(f.Info.Ident)
		} else {
			protoType = "google.protobuf.Any"
		}
	}
	optional := false
	if strings.HasPrefix(protoType, "google.protobuf.") {
		optional = f.Optional
	}
	fieldDesc := FieldDesc{
		ProtoType: protoType,
		Name:      f.Name,
		Index:     0,
		Comment:   f.Comment,
		Optional:  optional,
	}
	return fieldDesc
}
