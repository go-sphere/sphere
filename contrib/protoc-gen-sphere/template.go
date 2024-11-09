package main

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"
)

//go:embed template.go.tpl
var httpTemplate string

type serviceDesc struct {
	ServiceType string // Greeter
	ServiceName string // helloworld.Greeter
	Metadata    string // api/helloworld/helloworld.proto
	Methods     []*methodDesc
	MethodSets  map[string]*methodDesc
	Package     *packageDesc
}

type methodDesc struct {
	// method
	Name         string
	OriginalName string // The parsed original name
	Num          int
	Request      string
	Reply        string
	Comment      string
	// http_rule
	Path         string
	Method       string
	HasVars      bool
	HasQuery     bool
	HasBody      bool
	Body         string
	ResponseBody string
	// Temp
	Swagger      string
	GinPath      string
	NeedValidate bool
}

type packageDesc struct {
	RouterType  string
	ContextType string

	DataResponseType  string
	ErrorResponseType string

	ServerHandlerWrapperFunc string

	ParseJsonFunc string
	ParseUriFunc  string
	ParseFormFunc string

	ValidateFunc string
}

func (s *serviceDesc) execute() string {
	s.MethodSets = make(map[string]*methodDesc)
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
