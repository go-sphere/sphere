package boot

import (
	"flag"
	"fmt"
	"os"
)

// DefaultConfigParser provides a standard command-line configuration parser with version and help flags.
// It handles common CLI flags (config, version, help) and parses configuration from the specified file.
// The parser function should read and parse the configuration file into type T.
func DefaultConfigParser[T any](ver string, parser func(string) (*T, error)) *T {
	path := flag.String("config", "config.json", "config file path")
	version := flag.Bool("version", false, "show version")
	help := flag.Bool("help", false, "show help")
	flag.Parse()

	if *version {
		versionPrinter(ver)
		os.Exit(0)
	}

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	conf, err := parser(*path)
	if err != nil {
		fmt.Println("load config error: ", err)
		os.Exit(1)
	}
	return conf
}
