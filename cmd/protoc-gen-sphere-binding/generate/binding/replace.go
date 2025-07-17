package binding

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/fatih/structtag"
)

type StructTags map[string]map[string]*structtag.Tags

func ReTags(file *ast.File, tags StructTags) error {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		var typeSpec *ast.TypeSpec
		for _, spec := range genDecl.Specs {
			if ts, tsOK := spec.(*ast.TypeSpec); tsOK {
				typeSpec = ts
				break
			}
		}
		if typeSpec == nil {
			continue
		}

		structDecl, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}

		structName := typeSpec.Name.String()
		fieldsToRetag, structFound := tags[structName]
		if !structFound {
			continue
		}

		for _, field := range structDecl.Fields.List {
			for _, fieldName := range field.Names {
				newTags, fieldFound := fieldsToRetag[fieldName.String()]
				if !fieldFound || newTags == nil {
					continue
				}

				if field.Tag == nil {
					field.Tag = &ast.BasicLit{Kind: token.STRING}
				}

				currentTagValue := strings.Trim(field.Tag.Value, "`")
				oldTags, parseErr := structtag.Parse(currentTagValue)
				if parseErr != nil {
					return parseErr
				}

				sort.Stable(newTags)
				for _, t := range newTags.Tags() {
					if setErr := oldTags.Set(t); setErr != nil {
						return setErr
					}
				}
				field.Tag.Value = "`" + oldTags.String() + "`"
			}
		}
	}
	return nil
}
