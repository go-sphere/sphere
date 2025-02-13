package main

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"strings"
)

func RenameModule(oldModule, newModule string, path string) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		log.Printf("parse file error: %v", err)
		return err
	}
	ast.Inspect(node, func(n ast.Node) bool {
		importSpec, ok := n.(*ast.ImportSpec)
		if ok {
			goPath := strings.Trim(importSpec.Path.Value, `"`)
			if strings.HasPrefix(goPath, oldModule) {
				newPath := strings.Replace(goPath, oldModule, newModule, 1)
				importSpec.Path.Value = `"` + newPath + `"`
			}
		}
		return true
	})
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("open file error: %v", err)
		return err
	}
	defer file.Close()
	err = printer.Fprint(file, fset, node)
	if err != nil {
		log.Printf("write file error: %v", err)
		return err
	}
	return nil
}
