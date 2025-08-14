//go:build spheretools
// +build spheretools

package main

import (
	"flag"
	"log"
	"os"

	"github.com/TBXark/sphere/database/bind"
	"github.com/TBXark/sphere/layout/api/entpb"
	sharedv1 "github.com/TBXark/sphere/layout/api/shared/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/admin"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/adminsession"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/keyvaluestore"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/user"
	"golang.org/x/tools/imports"
)

func main() {
	file := flag.String("file", "./internal/pkg/render/bind.go", "file path")
	flag.Parse()
	if *file == "" {
		log.Fatal("file is required")
	}
	content, err := bind.GenFile(bindItems())
	if err != nil {
		log.Fatalf("generate bind code failed: %v", err)
	}
	formattedSrc, err := imports.Process(*file, []byte(content), nil)
	if err != nil {
		log.Fatalf("format code failed: %v", err)
	}
	err = os.WriteFile(*file, formattedSrc, 0o644)
	if err != nil {
		log.Fatalf("write file failed: %v", err)
	}
}

func bindItems() *bind.GenFileConf {
	return &bind.GenFileConf{
		Entities: []bind.GenFileEntityConf{
			{
				Actions: []any{ent.AdminCreate{}, ent.AdminUpdateOne{}},
				ConfigBuilder: func(act any) *bind.GenFuncConf {
					return bind.NewGenFuncConf(ent.Admin{}, entpb.Admin{}, act).
						WithIgnoreFields(admin.FieldCreatedAt, admin.FieldUpdatedAt)
				},
			},
			{
				Actions: []any{ent.AdminSessionCreate{}, ent.AdminSessionUpdateOne{}},
				ConfigBuilder: func(act any) *bind.GenFuncConf {
					return bind.NewGenFuncConf(ent.AdminSession{}, entpb.AdminSession{}, act).
						WithIgnoreFields(adminsession.FieldCreatedAt, adminsession.FieldUpdatedAt)
				},
			},
			{
				Actions: []any{ent.UserCreate{}, ent.UserUpdateOne{}},
				ConfigBuilder: func(act any) *bind.GenFuncConf {
					return bind.NewGenFuncConf(ent.User{}, sharedv1.User{}, act).
						WithIgnoreFields(user.FieldCreatedAt, user.FieldUpdatedAt)
				},
			},
			{
				Actions: []any{ent.KeyValueStoreCreate{}, ent.KeyValueStoreUpdateOne{}, ent.KeyValueStoreUpsertOne{}},
				ConfigBuilder: func(act any) *bind.GenFuncConf {
					return bind.NewGenFuncConf(ent.KeyValueStore{}, entpb.KeyValueStore{}, act).
						WithIgnoreFields(keyvaluestore.FieldCreatedAt, keyvaluestore.FieldUpdatedAt)
				},
			},
		},
	}
}
