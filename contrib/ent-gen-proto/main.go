package main

import (
	"entgo.io/contrib/entproto"
	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
	"flag"
	"log"
	"path/filepath"
	"sort"
)

func main() {
	var (
		schemaPath        = flag.String("path", "./internal/pkg/database/ent/schema", "path to schema directory")
		protoDir          = flag.String("proto", "./proto", "path to proto directory")
		ignoreOptional    = flag.Bool("ignore-optional", true, "ignore optional keyword")
		autoAddAnnotation = flag.Bool("auto-annotation", true, "auto add annotation to the schema")
	)
	flag.Parse()
	RunProtoGen(*schemaPath, *protoDir, *ignoreOptional, *autoAddAnnotation)
}

func RunProtoGen(schemaPath string, protoDir string, ignoreOptional bool, autoAddAnnotation bool) {
	abs, err := filepath.Abs(schemaPath)
	if err != nil {
		log.Fatalf("entproto: failed getting absolute path: %v", err)
	}
	graph, err := entc.LoadGraph(schemaPath, &gen.Config{
		Target: filepath.Dir(abs),
	})
	if err != nil {
		log.Fatalf("entproto: failed loading ent graph: %v", err)
	}
	if autoAddAnnotation {
		for i := 0; i < len(graph.Nodes); i++ {
			node := graph.Nodes[i]
			if node.Annotations == nil {
				node.Annotations = make(map[string]interface{}, 1)
			}
			if node.Annotations[entproto.MessageAnnotation] == nil {
				// If the node does not have the message annotation, add it.
				node.Annotations[entproto.MessageAnnotation] = entproto.Message()
				fieldID := 1
				if node.ID.Annotations == nil {
					node.ID.Annotations = make(map[string]interface{}, 1)
				}
				node.ID.Annotations[entproto.FieldAnnotation] = entproto.Field(fieldID)
				sort.Slice(node.Fields, func(i, j int) bool {
					if node.Fields[i].Position.MixedIn != node.Fields[j].Position.MixedIn {
						// MixedIn fields should be at the end of the list.
						return !node.Fields[i].Position.MixedIn
					}
					return node.Fields[i].Position.Index < node.Fields[j].Position.Index
				})

				for j := 0; j < len(node.Fields); j++ {
					field := node.Fields[j]
					if field.Annotations == nil {
						field.Annotations = make(map[string]interface{}, 1)
					}
					fieldID++
					field.Annotations[entproto.FieldAnnotation] = entproto.Field(fieldID)
					if field.Optional && ignoreOptional {
						field.Optional = false
					}
				}
			}
		}
	}
	extension, err := entproto.NewExtension(
		entproto.EnableOptional(),
		entproto.WithProtoDir(protoDir),
		entproto.SkipGenFile(),
	)
	if err != nil {
		log.Fatalf("entproto: failed creating entproto extension: %v", err)
	}
	err = extension.Generate(graph)
	if err != nil {
		log.Fatalf("entproto: failed generating protos: %s", err)
	}
}
