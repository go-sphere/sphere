package tags

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	rComment = regexp.MustCompile(`^//.*?@(?i:sphere?):\s*(.*)$`)
	rInject  = regexp.MustCompile("`.+`$")
	rTags    = regexp.MustCompile(`[\w_]+:"[^"]+"`)
	rAll     = regexp.MustCompile(".*")
)

type textArea struct {
	Start        int
	End          int
	CurrentTag   string
	InjectTag    string
	CommentStart int
	CommentEnd   int
}

func parseFile(inputPath string, src interface{}) ([]textArea, error) {
	files := token.NewFileSet()
	f, err := parser.ParseFile(files, inputPath, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	areas := make([]textArea, 0)
	for _, decl := range f.Decls {
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
		for _, field := range structDecl.Fields.List {
			var comments []*ast.Comment
			if field.Doc != nil {
				comments = append(comments, field.Doc.List...)
			}
			if field.Comment != nil {
				comments = append(comments, field.Comment.List...)
			}
			for _, comment := range comments {
				tag := tagFromComment(comment.Text)
				if tag == "" {
					continue
				}
				currentTag := field.Tag.Value
				area := textArea{
					Start:        int(field.Pos()),
					End:          int(field.End()),
					CurrentTag:   currentTag[1 : len(currentTag)-1],
					InjectTag:    tag,
					CommentStart: int(comment.Pos()),
					CommentEnd:   int(comment.End()),
				}
				areas = append(areas, area)
			}
		}
	}
	return areas, nil
}

func writeFile(inputPath string, areas []textArea, removeTagComment bool) error {
	file, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	err = file.Close()
	if err != nil {
		return err
	}
	for i := range areas {
		area := areas[len(areas)-i-1]
		contents = injectTag(contents, area, removeTagComment)
	}
	contents, err = format.Source(contents)
	if err != nil {
		return err
	}
	err = os.WriteFile(inputPath, contents, 0o644)
	if err != nil {
		return err
	}
	return nil
}

func ReTags(inputPath string, removeTagComment bool) error {
	if inputPath == "" {
		return nil
	}
	globResults, gErr := filepath.Glob(inputPath)
	if gErr != nil {
		return gErr
	}
	for _, path := range globResults {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".go") {
			continue
		}
		log.Printf("retags file: %s", path)
		areas, err := parseFile(path, nil)
		if err != nil {
			return err
		}
		if len(areas) == 0 {
			continue
		}
		err = writeFile(path, areas, removeTagComment)
		if err != nil {
			return err
		}
	}
	return nil
}
