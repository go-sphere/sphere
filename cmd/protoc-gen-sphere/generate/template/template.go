package template

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"text/template"
)

//go:embed template.go.tpl
var httpTemplate string

type ServiceDesc struct {
	ServiceType string
	ServiceName string
	Metadata    string
	Methods     []*MethodDesc
	MethodSets  map[string]*MethodDesc
	Package     *PackageDesc
}

type MethodDesc struct {
	// method
	Name         string
	OriginalName string
	Num          int
	Comment      string

	Request  string // rpc request type
	Reply    string // rpc reply type
	Response string // http response type

	// http_rule
	Path         string
	Method       string
	HasVars      bool
	HasQuery     bool
	HasBody      bool
	Body         string
	ResponseBody string

	// temp
	Swagger      string
	GinPath      string
	NeedValidate bool
}

type PackageDesc struct {
	RouterType  string
	ContextType string

	ErrorResponseType string
	DataResponseType  string

	ServerHandlerWrapperFunc string

	ParseJsonFunc string
	ParseUriFunc  string
	ParseFormFunc string

	ValidateFunc string
}

func (s *ServiceDesc) Execute() string {
	s.MethodSets = make(map[string]*MethodDesc)
	for _, m := range s.Methods {
		s.MethodSets[m.Name] = m
	}
	buf := new(bytes.Buffer)
	tmpl, err := template.New("http").Parse(strings.TrimSpace(httpTemplate))
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(buf, s)
	if err != nil {
		panic(err)
	}
	return strings.Trim(buf.String(), "\r\n")
}

func ReplaceTemplateIfNeed(path string) {
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "read template file error: %v\n", err)
			os.Exit(2)
		}
		httpTemplate = string(raw)
	}
}
