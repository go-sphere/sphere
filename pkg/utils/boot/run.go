package boot

import (
	"flag"
	"fmt"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/log/logfields"
	"os"
	"time"
)

const DefaultTimezone = "Asia/Shanghai"

func init() {
	_ = InitTimezone(DefaultTimezone)
}

func InitTimezone(zone string) error {
	defaultLoc := "Asia/Shanghai"
	loc, err := time.LoadLocation(defaultLoc)
	if err != nil {
		return err
	}
	time.Local = loc
	return os.Setenv("TZ", defaultLoc)
}

func DefaultConfigParser[T any](ver string, parser func(string) (*T, error)) *T {
	path := flag.String("config", "config.json", "config file path")
	version := flag.Bool("version", false, "show version")
	help := flag.Bool("help", false, "show help")
	flag.Parse()

	if *version {
		fmt.Println(ver)
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

func Run[T any](ver string, conf *T, logConf *log.Options, builder func(*T) (*Application, error)) error {
	log.Init(logConf, logfields.String("version", ver))
	log.Info("Start application", logfields.String("version", ver))
	defer func() {
		if e := log.Sync(); e != nil {
			fmt.Println("log sync error: ", e)
		}
	}()
	app, err := builder(conf)
	if err != nil {
		return err
	}
	defer app.Clean()
	app.Run()
	return nil
}
