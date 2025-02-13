package main

import (
	"flag"
	"github.com/TBXark/sphere/contrib/import-renamer/renamer"
	"log"
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
	err := renamer.RenameDirModule(*oldModule, *newModule, *target)
	if err != nil {
		log.Panicf("rename module error: %v", err)
	}
}
