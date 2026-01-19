package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sunvim/evm_executor/pkg/config"
	"github.com/sunvim/evm_executor/pkg/executor"
	"github.com/sunvim/evm_executor/pkg/logger"
	"github.com/sunvim/evm_executor/pkg/metrics"
)

var (
	configPath = flag.String("config", "config/config.yaml", "Path to config file")
	version    = "dev"
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := logger.Initialize(cfg.Logging.Level, cfg.Logging.Format); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Infof("Starting EVM Executor version %s", version)
	logger.Infof("Chain: %s (ID: %d)", cfg.Chain.Name, cfg.Chain.NetworkID)

	// Start metrics server if enabled
	var metricsServer *metrics.Server
	if cfg.Metrics.Enabled {
		metricsServer = metrics.NewServer(cfg.Metrics.ListenAddr)
		if err := metricsServer.Start(); err != nil {
			logger.Fatalf("Failed to start metrics server: %v", err)
		}
		defer func() {
			if err := metricsServer.Stop(); err != nil {
				logger.Errorf("Failed to stop metrics server: %v", err)
			}
		}()
	}

	// Create executor
	exec, err := executor.NewExecutor(cfg)
	if err != nil {
		logger.Fatalf("Failed to create executor: %v", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start executor
	if err := exec.Start(ctx); err != nil {
		logger.Fatalf("Failed to start executor: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("EVM Executor started, waiting for blocks...")

	// Block until signal received
	sig := <-sigChan
	logger.Infof("Received signal %v, shutting down...", sig)

	// Cancel context
	cancel()

	// Stop executor
	if err := exec.Stop(); err != nil {
		logger.Errorf("Error stopping executor: %v", err)
		os.Exit(1)
	}

	logger.Info("EVM Executor shut down successfully")
}
