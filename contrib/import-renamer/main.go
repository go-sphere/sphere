package main

import (
	"flag"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	oldModule := flag.String("old", "", "old module name")
	newModule := flag.String("new", "", "new module name")
	target := flag.String("target", "", "target file")
	flag.Parse()

	if *oldModule == "" || *newModule == "" || *target == "" {
		flag.PrintDefaults()
		return
	}

	err := filepath.Walk(*target, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			log.Printf("rename file: %s", path)
			return renameModule(*oldModule, *newModule, path)
		}
		return nil
	})
	if err != nil {
		log.Panicf("rename module error: %v", err)
	}
}

func renameModule(oldModule, newModule string, path string) error {
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
