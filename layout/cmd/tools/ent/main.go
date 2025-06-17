//go:build spheratools
// +build spheratools

package main

import (
	"flag"
	"log"
	"path"
	"runtime/debug"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func main() {
	schema := flag.String("schema", "./internal/pkg/database/schema", "path to the schema directory")
	target := flag.String("target", "./internal/pkg/database/ent", "target directory for generated code")
	flag.Parse()
	err := entc.Generate(*schema, &gen.Config{
		Target:  *target,
		Package: path.Join(currentModule(), *target),
		Features: []gen.Feature{
			gen.FeatureModifier,
			gen.FeatureExecQuery,
			gen.FeatureUpsert,
			gen.FeatureLock,
		},
	})
	if err != nil {
		log.Fatal("running ent codegen:", err)
	}
}

func currentModule() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Path
}
