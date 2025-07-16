package binding

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/fatih/structtag"
)

type StructTags map[string]map[string]*structtag.Tags

func Retag(n ast.Node, tags StructTags) error {
	r := retag{}
	f := func(n ast.Node) ast.Visitor {
		if r.err != nil {
			return nil
		}
		if tp, ok := n.(*ast.TypeSpec); ok {
			r.tags = tags[tp.Name.String()]
			return r
		}
		return nil
	}
	ast.Walk(structVisitor{f}, n)
	return r.err
}

type structVisitor struct {
	visitor func(n ast.Node) ast.Visitor
}

func (v structVisitor) Visit(n ast.Node) ast.Visitor {
	if tp, exist := n.(*ast.TypeSpec); exist {
		if _, ok := tp.Type.(*ast.StructType); ok {
			ast.Walk(v.visitor(n), n)
			return nil // This will ensure this struct is no longer traversed
		}
	}
	return v
}

type retag struct {
	err  error
	tags map[string]*structtag.Tags
}

func (v retag) Visit(n ast.Node) ast.Visitor {
	if v.err != nil {
		return nil
	}

	if f, ok := n.(*ast.Field); ok {
		if len(f.Names) == 0 {
			return nil
		}
		newTags := v.tags[f.Names[0].String()]
		if newTags == nil {
			return nil
		}
		if f.Tag == nil {
			f.Tag = &ast.BasicLit{
				Kind: token.STRING,
			}
		}

		oldTags, err := structtag.Parse(strings.Trim(f.Tag.Value, "`"))
		if err != nil {
			v.err = err // nolint
			return nil
		}

		sort.Stable(newTags) // sort tags according to keys
		for _, t := range newTags.Tags() {
			_ = oldTags.Set(t)
		}

		f.Tag.Value = "`" + oldTags.String() + "`"
		return nil
	}
	return v
}
