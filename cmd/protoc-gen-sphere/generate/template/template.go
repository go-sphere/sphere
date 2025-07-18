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

/*
service TestService {
  rpc RunTest(RunTestRequest) returns (RunTestResponse) {
    option (google.api.http) = {
      post: "/api/test/{path_test1}/second/{path_test2}"
      body: "*"
    };
  }
}
*/

type ServiceDesc struct {
	ServiceType string // TestService
	ServiceName string // shared.v1.TestService

	Methods    []*MethodDesc
	MethodSets map[string]*MethodDesc

	Package *PackageDesc
}

type MethodDesc struct {
	// method
	Name         string // rpc method name: RunTest
	OriginalName string // service and method name: TestServiceRunTest
	Num          int    // duplicate method number, used for generating unique method names
	Comment      string // leading comment for the method

	Request  string // rpc request type
	Reply    string // rpc reply type
	Response string // http response type

	// http_rule
	Path   string // gin route: /api/test/:path_test1/second/:path_test
	Method string // POST

	HasVars      bool
	HasQuery     bool
	HasBody      bool
	NeedValidate bool

	Swagger string

	Body         string
	ResponseBody string
}

type PackageDesc struct {
	RouterType  string
	ContextType string

	ErrorResponseType string
	DataResponseType  string

	ParseJsonFunc string
	ParseUriFunc  string
	ParseFormFunc string
	ValidateFunc  string

	ServerHandlerWrapperFunc string
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
