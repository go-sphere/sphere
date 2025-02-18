package main

import (
	"flag"
	"github.com/TBXark/sphere/contrib/json-gen-proto/gen"
	"log"
	"os"
)

func main() {
	jsonFilePath := flag.String("input", "", "json file path")
	protoFilePath := flag.String("output", "", "proto file path")
	packageName := flag.String("package", "", "package name")
	rootMessage := flag.String("message", "", "root message name")
	help := flag.Bool("help", false, "help")
	flag.Parse()

	if *help {
		flag.PrintDefaults()
		return
	}

	if *jsonFilePath == "" || *protoFilePath == "" || *packageName == "" || *rootMessage == "" {
		flag.PrintDefaults()
		return
	}

	file, err := os.ReadFile(*jsonFilePath)
	if err != nil {
		log.Fatalf("read json file error: %v", err)
	}

	res, err := gen.Generate(*packageName, *rootMessage, file)
	if err != nil {
		log.Fatalf("generate proto error: %v", err)
	}

	err = os.WriteFile(*protoFilePath, []byte(res), 0644)
	if err != nil {
		log.Fatalf("write proto file error: %v", err)
	}
}
