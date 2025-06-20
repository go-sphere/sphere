//go:build spheretools
// +build spheretools

package main

import (
	"fmt"
	"os"

	"github.com/TBXark/sphere/core/boot"
	"github.com/TBXark/sphere/layout/internal/config"
	"github.com/TBXark/sphere/layout/internal/server/docs"
)

func main() {
	conf := boot.DefaultConfigParser(config.BuildVersion, config.NewConfig)
	err := boot.Run(conf, func(c *config.Config) (*boot.Application, error) {
		return boot.NewApplication(docs.NewWebServer(c.Docs)), nil
	})
	if err != nil {
		fmt.Printf("Boot error: %v", err)
		os.Exit(1)
	}
	fmt.Println("Boot done")
}
