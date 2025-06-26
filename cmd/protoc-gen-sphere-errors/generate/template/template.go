package template

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/TBXark/sphere/proto/errors/sphere/errors"
)

//go:embed template.go.tpl
var errorsTemplate string

type ErrorInfo struct {
	Name       string
	Comment    string
	HasComment bool
	Error      errors.Error
}

type ErrorWrapper struct {
	Errors []*ErrorInfo
}

func (e *ErrorWrapper) Execute() string {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("errors").Parse(errorsTemplate)
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(buf, e)
	if err != nil {
		panic(err)
	}
	return buf.String()
}
