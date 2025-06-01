package main

import (
	_ "embed"
	"flag"
	"log"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
)

func main() {
	pkg := flag.String("pkg", "dash.v1", "package name")
	name := flag.String("name", "Admin", "proto name")
	file := flag.String("file", "go", "generated file type, supports: proto, go")
	flag.Parse()

	generators := map[string]func(name, pkg string) (string, error){
		"proto": createProto,
		"go":    createGolangService,
	}

	genFunc, ok := generators[*file]
	if !ok {
		log.Panicf("Unsupported file type: %s, supported types are: %v", *file, []string{"proto", "go"})
	}
	proto, err := genFunc(*name, *pkg)
	if err != nil {
		log.Panic("Failed to create proto:", err)
	}
	log.Println("Generated Proto:\n", proto)
}

type ProtoConf struct {
	PackageName string
	ServiceName string
	RouteName   string
	EntityName  string
}

//go:embed proto.tpl
var protoTemplate string

func createProto(name, pkg string) (string, error) {
	conf := ProtoConf{
		PackageName: pkg,
		ServiceName: strcase.ToCamel(name),
		RouteName:   strcase.ToKebab(name),
		EntityName:  strcase.ToLowerCamel(name),
	}

	tmpl := template.New("proto")
	tmpl, err := tmpl.Parse(protoTemplate)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	err = tmpl.Execute(&sb, conf)
	if err != nil {
		return "", err
	}

	return sb.String(), nil
}

type GolangConf struct {
	ServiceName     string
	PackagePath     string
	ServicePackage  string
	BizPackagePath  string
	ServiceFileName string
}

//go:embed service.tpl
var serviceTemplate string

func createGolangService(name, pkg string) (string, error) {
	conf := GolangConf{
		ServiceName:     strcase.ToCamel(name),
		PackagePath:     strings.Join(strings.Split(pkg, "."), "/"),
		ServicePackage:  strings.ReplaceAll(pkg, ".", ""),
		BizPackagePath:  "github.com/TBXark/sphere/layout",
		ServiceFileName: strings.ToLower(name),
	}

	tmpl := template.New("service")
	tmpl, err := tmpl.Parse(serviceTemplate)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	err = tmpl.Execute(&sb, conf)
	if err != nil {
		return "", err
	}

	return sb.String(), nil
}
