package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	oldModule := flag.String("old", "", "old module name")
	newModule := flag.String("new", "", "new module name")
	target := flag.String("target", "", "target file")
	flag.Parse()

	if *oldModule == "" || *newModule == "" || *target == "" {
		flag.PrintDefaults()
		return
	}

	err := filepath.Walk(*target, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			log.Printf("rename file: %s", path)
			return RenameModule(*oldModule, *newModule, path)
		}
		return nil
	})
	if err != nil {
		log.Panicf("rename module error: %v", err)
	}
}
