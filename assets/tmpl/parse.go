//go:build !tmplGen

package tmpl

import (
	"path"
	"reflect"
	"strings"
	"text/template"
)

func ParseTemplates() (*List, error) {
	var tmpl List
	files, err := Assets.ReadDir(".")
	if err != nil {
		return nil, err
	}
	mutable := reflect.ValueOf(&tmpl).Elem()

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		b, e := Assets.ReadFile(path.Join(AssetsDir, file.Name()))
		if e != nil {
			return nil, e
		}
		t, e := template.New(file.Name()).Parse(string(b))
		if e != nil {
			return nil, e
		}
		fieldName := file.Name()[:len(file.Name())-5]
		fieldName = strings.ToUpper(fieldName[:1]) + fieldName[1:]
		mutable.FieldByName(fieldName).Set(reflect.ValueOf(t))
	}
	return &tmpl, nil
}

func Execute(t *template.Template, data any) (string, error) {
	var sb strings.Builder
	err := t.Execute(&sb, data)
	return sb.String(), err
}
