package boot

import (
	"flag"
	"fmt"
	"os"
)

// DefaultConfigParser parses -config (default config.json), -version, and -help
// via flag.Parse. It calls os.Exit on -version, -help, and parse failure — it
// is a process entry helper, not a library-friendly parser. parser reads the
// file at the resolved path into T.
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
