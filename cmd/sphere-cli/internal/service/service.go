package service

import (
	_ "embed"
	"go/format"
	"strings"
	"text/template"

	"github.com/go-openapi/inflect"
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

func GenServiceGolang(name, pkg, mod string) (string, error) {
	conf := serviceConfig{
		ServiceName:     strcase.ToCamel(name),
		PackagePath:     strings.Join(strings.Split(pkg, "."), "/"),
		ServicePackage:  strings.ReplaceAll(pkg, ".", ""),
		BizPackagePath:  mod,
		ServiceFileName: strings.ToLower(name),
	}
	rules := inflect.NewDefaultRuleset()

	tmpl := template.New("service").Funcs(template.FuncMap{
		"plural": rules.Pluralize,
	})
	tmpl, err := tmpl.Parse(serviceTemplate)
	if err != nil {
		return "", err
	}

	var file strings.Builder
	err = tmpl.Execute(&file, conf)
	if err != nil {
		return "", err
	}

	source, err := format.Source([]byte(file.String()))
	if err != nil {
		return "", err
	}
	return string(source), nil
}
