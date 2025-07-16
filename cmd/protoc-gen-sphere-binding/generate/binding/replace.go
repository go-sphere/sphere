package binding

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/fatih/structtag"
)

type StructTags map[string]map[string]*structtag.Tags

func ReTags(n ast.Node, tags StructTags) error {
	var err error
	ast.Inspect(n, func(node ast.Node) bool {
		if err != nil {
			return false
		}

		typeSpec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		structName := typeSpec.Name.String()
		fieldsToRetag, structFound := tags[structName]
		if !structFound {
			return true
		}

		for _, field := range structType.Fields.List {
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
					err = parseErr
					return false
				}

				sort.Stable(newTags)
				for _, t := range newTags.Tags() {
					if setErr := oldTags.Set(t); setErr != nil {
						err = setErr
						return false
					}
				}

				field.Tag.Value = "`" + oldTags.String() + "`"
			}
		}

		return false
	})

	return err
}
