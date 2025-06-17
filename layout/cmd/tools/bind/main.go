//go:build spheratools
// +build spheratools

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"github.com/TBXark/sphere/database/bind"
	"github.com/TBXark/sphere/layout/api/entpb"
	sharedv1 "github.com/TBXark/sphere/layout/api/shared/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/admin"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/keyvaluestore"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/user"
)

func main() {
	file := flag.String("file", "./internal/pkg/render/bind.go", "file path")
	mod := flag.String("mod", currentModule(), "go module path")
	flag.Parse()
	if *file == "" {
		log.Fatal("file is required")
	}
	if *mod == "" {
		log.Fatal("mod is required")
	}
	content, err := bind.GenFile(*mod, bindItems(*mod))
	if err != nil {
		log.Fatalf("generate bind code failed: %v", err)
	}
	err = os.WriteFile(*file, []byte(content), 0o644)
	if err != nil {
		log.Fatalf("write file failed: %v", err)
	}
}

func currentModule() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Path
}

func bindItems(mod string) *bind.GenFileConf {
	return &bind.GenFileConf{
		ExtraImports: [][]string{
			{fmt.Sprintf("%s/api/shared/v1", mod), "sharedv1"},
		},
		Entities: []bind.GenFileEntityConf{
			{
				Entity:  ent.Admin{},
				Actions: []any{ent.AdminCreate{}, ent.AdminUpdateOne{}},
				ConfigBuilder: func(act any) *bind.GenFuncConf {
					return bind.NewGenFuncConf(ent.Admin{}, entpb.Admin{}, act).
						WithIgnoreFields(admin.FieldCreatedAt, admin.FieldUpdatedAt)
				},
			},
			{
				Entity:  ent.User{},
				Actions: []any{ent.UserCreate{}, ent.UserUpdateOne{}},
				ConfigBuilder: func(act any) *bind.GenFuncConf {
					return bind.NewGenFuncConf(ent.User{}, sharedv1.User{}, act).
						WithIgnoreFields(user.FieldCreatedAt, user.FieldUpdatedAt).
						WithTargetPkgName("sharedv1")
				},
			},
			{
				Entity:  ent.KeyValueStore{},
				Actions: []any{ent.KeyValueStoreCreate{}, ent.KeyValueStoreUpdateOne{}, ent.KeyValueStoreUpsertOne{}},
				ConfigBuilder: func(act any) *bind.GenFuncConf {
					return bind.NewGenFuncConf(ent.KeyValueStore{}, entpb.KeyValueStore{}, act).
						WithIgnoreFields(keyvaluestore.FieldCreatedAt, keyvaluestore.FieldUpdatedAt)
				},
			},
		},
	}
}
