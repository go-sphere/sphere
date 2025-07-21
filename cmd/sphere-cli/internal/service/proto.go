package service

import (
	_ "embed"
	"github.com/go-openapi/inflect"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
)

type protoConfig struct {
	PackageName string
	ServiceName string
	RouteName   string
	EntityName  string
}

//go:embed proto.tpl
var protoTemplate string

func GenServiceProto(name, pkg string) (string, error) {
	conf := protoConfig{
		PackageName: pkg,
		ServiceName: strcase.ToCamel(name),
		RouteName:   strcase.ToKebab(name),
		EntityName:  strcase.ToSnake(name),
	}
	rules := inflect.NewDefaultRuleset()

	tmpl := template.New("proto").Funcs(template.FuncMap{
		"plural": rules.Pluralize,
	})
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
