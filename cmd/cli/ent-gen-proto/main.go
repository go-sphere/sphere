package main

import (
	"entgo.io/ent/entc/load"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func main() {
	path := flag.String("path", "./internal/pkg/database/ent/schema", "path to the schema directory")
	protoPath := flag.String("proto", "./proto/data/v1/model.proto", "path to the generated proto file")
	protoPackage := flag.String("package", "data.v1", "package name for the generated proto file")
	flag.Parse()
	spec, err := localSchemaSpec(path)
	if err != nil {
		log.Panic(err)
	}
	fileDesc := genFileDesc(protoPackage, spec)
	file, err := createProtoFile(err, *protoPath)
	defer file.Close()
	parse, err := template.New("proto").Parse(strings.TrimSpace(protoTpl))
	if err != nil {
		log.Panic(err)
	}
	err = parse.Execute(file, fileDesc)
	if err != nil {
		log.Panic(err)
	}
}

func localSchemaSpec(path *string) (*load.SchemaSpec, error) {
	schemaPath, err := filepath.Abs(*path)
	if err != nil {
		log.Panic(err)
	}
	log.Printf("loading schema from %s", schemaPath)
	config := load.Config{
		Path:       schemaPath,
		Names:      nil,
		BuildFlags: nil,
	}
	spec, err := config.Load()
	if err != nil {
		log.Panic(err)
	}
	return spec, err
}

func createProtoFile(err error, protoPath string) (*os.File, error) {
	protoFile, err := filepath.Abs(protoPath)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(filepath.Dir(protoFile), os.ModePerm)
	if err != nil {
		return nil, err
	}
	file, err := os.Create(protoFile)
	if err != nil {
		return nil, err
	}
	return file, nil
}
