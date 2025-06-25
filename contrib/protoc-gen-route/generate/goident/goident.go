package goident

import (
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

type GoIdent struct {
	Path   protogen.GoImportPath
	Iden   string
	IsFunc bool
}

func (g GoIdent) GoIdent() protogen.GoIdent {
	return g.Path.Ident(g.Iden)
}

func NewGoIdent(s string, options ...func(*GoIdent)) *GoIdent {
	parts := strings.Split(s, ";")
	if len(parts) != 2 {
		return nil
	}
	ident := &GoIdent{
		Path: protogen.GoImportPath(parts[0]),
		Iden: parts[1],
	}
	for _, opt := range options {
		opt(ident)
	}
	return ident
}

func IsFunc() func(*GoIdent) {
	return func(g *GoIdent) {
		g.IsFunc = true
	}
}
