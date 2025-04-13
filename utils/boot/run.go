package boot

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
)

const DefaultTimezone = "Asia/Shanghai"

var versionPrinter = func(version string) {
	fmt.Println(version)
}

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

func InitVersionPrinter(printer func(string)) {
	versionPrinter = printer
}

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

func Run[T any](ver string, conf *T, logConf *log.Options, builder func(*T) (*Application, error)) error {
	// Init logger
	log.Init(logConf, logfields.String("version", ver))
	log.Info("Start application", logfields.String("version", ver))
	defer func() {
		_ = log.Sync()
	}()

	// Create application
	app, err := builder(conf)
	if err != nil {
		return fmt.Errorf("failed to build application: %w", err)
	}

	// Create root context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	// Start application
	errChan := make(chan error, 1)
	go func() {
		errChan <- app.Start(ctx)
	}()

	// Wait for shutdown signal or application error
	var errs []error
	select {
	case sig := <-quit:
		log.Infof("Received shutdown signal: %v", sig)
		cancel() // Trigger application shutdown
	case e := <-errChan:
		if e != nil {
			log.Error("Application error", logfields.Error(e))
			errs = append(errs, fmt.Errorf("application error: %w", e))
			cancel() // Ensure context is canceled
		}
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	err = app.Stop(shutdownCtx)
	if err != nil {
		errs = append(errs, fmt.Errorf("shutdown error: %w", err))
	}
	return errors.Join(errs...)
}
