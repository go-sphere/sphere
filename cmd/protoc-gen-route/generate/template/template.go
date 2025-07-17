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
var routeTemplate string

type ServiceDesc struct {
	OptionsKey  string
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
	Request      string
	Reply        string
	Comment      string
	// extra data
	Extra map[string]string
}

type PackageDesc struct {
	RequestType      string
	ResponseType     string
	ExtraDataType    string
	NewExtraDataFunc string
}

func (s *ServiceDesc) Execute() string {
	s.MethodSets = make(map[string]*MethodDesc)
	for _, m := range s.Methods {
		s.MethodSets[m.Name] = m
	}
	buf := new(bytes.Buffer)
	tmpl, err := template.New("route").Parse(strings.TrimSpace(routeTemplate))
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
		routeTemplate = string(raw)
	}
}
