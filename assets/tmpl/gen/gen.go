//go:build tmplGen
// +build tmplGen

package main

import (
	"github.com/tbxark/go-base-api/assets/tmpl"
	"log"

	"os"
	"path"
	"strings"
)

func GenTemplatesStruct() (string, error) {
	var sb strings.Builder
	sb.WriteString(`//go:build !tmplGen
package tmpl

import "text/template"

type List struct {
`)
	files, err := tmpl.Assets.ReadDir(tmpl.AssetsDir)
	if err != nil {
		return "", err
	}
	list := make([]string, 0, len(files))
	maxLen := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fieldName := file.Name()[:len(file.Name())-5]
		fieldName = strings.ToUpper(fieldName[:1]) + fieldName[1:]
		list = append(list, fieldName)
		if len(fieldName) > maxLen {
			maxLen = len(fieldName)
		}
	}
	for _, fieldName := range list {
		sb.WriteString("\t")
		sb.WriteString(fieldName)
		sb.WriteString(strings.Repeat(" ", maxLen-len(fieldName)))
		sb.WriteString(" *template.Template\n")
	}
	sb.WriteString("}\n")
	return sb.String(), nil
}

func main() {
	dirname := os.Args[1]
	tmpl, err := GenTemplatesStruct()
	if err != nil {
		log.Panicf("generate template struct error: %v", err)
	}
	target := path.Join(dirname, "list.go")
	err = os.WriteFile(target, []byte(tmpl), 0644)
	if err != nil {
		log.Panicf("write template struct error: %v", err)
	}
}
