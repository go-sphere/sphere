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

/*
service MenuService {
  // test comment line1
  // test comment line2
  // test comment line3
  rpc UpdateCount(UpdateCountRequest) returns (UpdateCountResponse) {
    option (sphere.options.options) = {
      key: "bot"
      extra: [
        {
          key: "command"
          value: "start"
        },
        {
          key: "callback_query"
          value: "start"
        }
      ]
    };
  }
}
*/

type ServiceDesc struct {
	OptionsKey string // bot

	ServiceType string // MenuService
	ServiceName string // bot.v1.MenuService

	Methods    []*MethodDesc
	MethodSets map[string]*MethodDesc

	Package *PackageDesc
}

type MethodDesc struct {
	Name         string // rpc method name: UpdateCount
	OriginalName string // service and method name: MenuServiceUpdateCount
	Num          int    // duplicate method number, used for generating unique method names

	Request string // rpc request type: UpdateCountRequest
	Reply   string // rpc reply type: UpdateCountResponse
	Comment string

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
