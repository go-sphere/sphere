package protogo

import (
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

type GoIdent struct {
	Path         protogen.GoImportPath
	Ident        string
	IsFunc       bool
	GenericCount int
}

func (g GoIdent) GoIdent() protogen.GoIdent {
	return g.Path.Ident(g.Ident)
}

func NewGoIdent(s string, options ...func(*GoIdent)) *GoIdent {
	parts := strings.Split(s, ";")
	if len(parts) != 2 {
		return nil
	}
	i := &GoIdent{
		Path:  protogen.GoImportPath(parts[0]),
		Ident: parts[1],
	}
	for _, option := range options {
		option(i)
	}
	return i
}

func IsFunc() func(*GoIdent) {
	return func(g *GoIdent) {
		g.IsFunc = true
	}
}

func GenericCount(count int) func(*GoIdent) {
	return func(g *GoIdent) {
		g.GenericCount = count
	}
}
