package boot

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
	"os"
	"os/signal"
	"syscall"
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
	// Init logger
	log.Init(logConf, logfields.String("version", ver))
	log.Info("Start application", logfields.String("version", ver))
	defer func() {
		if e := log.Sync(); e != nil {
			log.Warnf("Failed to sync log: %v", e)
		}
	}()

	// Create application
	app, err := builder(conf)
	if err != nil {
		return fmt.Errorf("failed to build application: %w", err)
	}

	// Listen for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	// Catch application error
	errChan := make(chan error, 1)
	go func() {
		errChan <- app.Start(context.Background())
	}()

	errs := make([]error, 0, 2)

	// Wait for shutdown signal or application error
	select {
	case <-quit:
		log.Debug("Received shutdown signal")
	case e := <-errChan:
		if e != nil {
			log.Error("Application error", logfields.Error(e))
			errs = append(errs, e)
		}
	}

	// Close application
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if e := app.Stop(ctx); e != nil {
		errs = append(errs, e)
	}

	return errors.Join(errs...)
}
