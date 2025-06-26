package template

import (
	"bytes"
	_ "embed"
	"text/template"
)

//go:embed template.go.tpl
var errorsTemplate string

type ErrorInfo struct {
	Name       string
	Value      string
	CamelValue string

	HasMessage bool

	Status  int32
	Code    int32
	Reason  string
	Message string
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
