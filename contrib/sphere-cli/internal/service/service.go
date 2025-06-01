package service

import (
	_ "embed"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
)

type serviceConfig struct {
	ServiceName     string
	PackagePath     string
	ServicePackage  string
	BizPackagePath  string
	ServiceFileName string
}

//go:embed service.tpl
var serviceTemplate string

func GenServiceGolang(name, pkg string) (string, error) {
	conf := serviceConfig{
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
