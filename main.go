package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"modbusslave-test/config"
	"modbusslave-test/datastore"
	"modbusslave-test/poller"
	"modbusslave-test/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	logger := setupLogger(cfg.Logging.Level)

	logger.Info("application starting",
		"source", cfg.SourceAddress(),
		"server", cfg.ServerAddress(),
		"blocks", len(cfg.RegisterBlocks),
	)

	for _, b := range cfg.RegisterBlocks {
		logger.Info("register block",
			"name", b.Name,
			"function", b.Function,
			"start", b.StartAddress,
			"count", b.Count,
		)
	}

	store := datastore.New()

	srv := server.New(cfg, store, logger)
	if err := srv.Start(); err != nil {
		logger.Error("failed to start slave server", "error", err)
		os.Exit(1)
	}

	p := poller.New(cfg, store, logger)
	if err := p.Start(); err != nil {
		logger.Error("failed to start poller", "error", err)
		srv.Stop()
		os.Exit(1)
	}

	logger.Info("application ready - PLC mirror active",
		"source", cfg.SourceAddress(),
		"slave", cfg.ServerAddress(),
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info("shutdown signal received", "signal", sig.String())

	p.Stop()
	srv.Stop()
	logger.Info("application stopped")
}

func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	return slog.New(handler)
}
