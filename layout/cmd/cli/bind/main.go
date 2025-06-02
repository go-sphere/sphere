package main

import (
	"flag"
	"go/format"
	"log"
	"os"

	"github.com/TBXark/sphere/database/bind"
	"github.com/TBXark/sphere/layout/api/entpb"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/admin"
)

func main() {
	file := flag.String("file", "./internal/pkg/render/bind.go", "file path")
	mod := flag.String("mod", "", "go module path")
	flag.Parse()
	if *file == "" {
		log.Fatal("file is required")
	}
	if *mod == "" {
		log.Fatal("mod is required")
	}
	content, err := bind.GenFile(*mod, bindItems())
	if err != nil {
		log.Fatalf("generate bind code failed: %v", err)
	}
	source, err := format.Source([]byte(content))
	if err != nil {
		log.Fatalf("format source failed: %v", err)
	}
	err = os.WriteFile(*file, source, 0o644)
	if err != nil {
		log.Fatalf("write file failed: %v", err)
	}
}

func bindItems() []bind.GenFileConf {
	return []bind.GenFileConf{
		{
			Entity:  ent.Admin{},
			Actions: []any{ent.AdminCreate{}, ent.AdminUpdateOne{}},
			ConfigBuilder: func(act any) *bind.GenFuncConf {
				return bind.NewGenFuncConf(ent.Admin{}, entpb.Admin{}, act).
					WithIgnoreFields(admin.FieldCreatedAt, admin.FieldUpdatedAt)
			},
		},
	}
}
