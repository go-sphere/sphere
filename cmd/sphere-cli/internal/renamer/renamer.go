package renamer

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func RenameDirModule(oldModule, newModule string, dir string) error {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			log.Printf("rename file: %s", path)
			return RenameModule(oldModule, newModule, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func RenameModule(oldModule, newModule string, path string) error {
	files := token.NewFileSet()
	node, err := parser.ParseFile(files, path, nil, parser.ParseComments)
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
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		log.Printf("open file error: %v", err)
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	err = printer.Fprint(file, files, node)
	if err != nil {
		log.Printf("write file error: %v", err)
		return err
	}
	return nil
}
