package template

import (
	_ "embed"
	"strings"
	"text/template"
)

//go:embed template.tmpl
var errorsTemplate string

type ErrorInfo struct {
	Name       string
	Value      string
	CamelValue string

	Status  int32
	Code    int32
	Reason  string
	Message string
}

func (i *ErrorInfo) HasReason() bool {
	return i.Reason != ""
}

func (i *ErrorInfo) HasMessage() bool {
	return i.Message != ""
}

type ErrorWrapper struct {
	Name   string
	Errors []*ErrorInfo
}

func (e *ErrorWrapper) Execute() (string, error) {
	var buf strings.Builder
	tmpl, err := template.New("errors").Parse(errorsTemplate)
	if err != nil {
		return "", err
	}
	err = tmpl.Execute(&buf, e)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
