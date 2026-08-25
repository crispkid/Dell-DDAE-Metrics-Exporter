package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/app"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

var (
	version   = "dev"
	revision  = "unknown"
	buildDate = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ddae-exporter failed")
		os.Exit(1)
	}
}

func run() error {
	return runWithArgs(os.Args[1:])
}

func runWithArgs(arguments []string) error {
	flags := flag.NewFlagSet("ddae-exporter", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configurationPath := flags.String("config", "", "path to versioned YAML configuration")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return fmt.Errorf("invalid command-line arguments")
	}
	configurationPathSet := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == "config" {
			configurationPathSet = true
		}
	})
	if configurationPathSet && *configurationPath == "" {
		return fmt.Errorf("configuration file path is empty")
	}
	cfg, err := config.LoadFile(*configurationPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		return err
	}
	logger := observability.NewLogger(os.Stdout, cfg.LogLevel, cfg.LogFormat)
	warnInsecureTLS(logger, cfg)
	application, err := app.New(cfg, logger, app.BuildInfo{Version: version, Revision: revision, BuildDate: buildDate})
	if err != nil {
		observability.LogFailure(logger, "startup failed", "startup", err)
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := application.Run(ctx); err != nil {
		observability.LogFailure(logger, "runtime failed", "runtime", err)
		return err
	}
	return nil
}

func warnInsecureTLS(logger interface{ Warn(string, ...any) }, cfg config.Config) {
	for _, target := range cfg.InsecureTLSTargets() {
		logger.Warn("TLS certificate and hostname verification disabled", "component", "tls", "target", target)
	}
}
